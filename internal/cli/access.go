package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
	"jira-flow.local/jflow/internal/provider/jiracloud"
	"jira-flow.local/jflow/internal/secretstore"
)

// Dependencies allow command tests to exercise full flows without a real keyring.
type Dependencies struct {
	Env     func(string) string
	Secrets ports.SecretStore
	Jira    app.IdentityClient
}

func addAccess(root *cobra.Command, deps Dependencies, emit func(any, string) error) {
	var profile, email string
	var noInput bool
	root.PersistentFlags().StringVar(&profile, "profile", "", "Perfil para esta invocación")
	root.PersistentFlags().StringVar(&email, "email", "", "Correo de Atlassian para esta invocación")
	root.PersistentFlags().BoolVar(&noInput, "no-input", false, "No solicitar entrada interactiva")
	access := func() (app.Access, error) {
		env := deps.Env
		if env == nil {
			env = os.Getenv
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return app.Access{}, &domain.Error{Kind: domain.InvalidInput, Message: "No se pudo resolver el directorio personal."}
		}
		paths := config.ResolvePaths(runtime.GOOS, home, env)
		store := deps.Secrets
		if store == nil {
			store = secretstore.New()
		}
		jira := deps.Jira
		if jira == nil {
			jira = defaultIdentity{CAFile: env("JFLOW_CA_CERT")}
		}

		return app.Access{Path: paths.Config, Env: env, Secrets: store, Jira: jira}, nil
	}
	command := func(use, short string, run func(*cobra.Command, []string) error) *cobra.Command {
		return &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: run}
	}
	auth := &cobra.Command{Use: "auth", Short: "Autenticación personal con Jira Cloud"}
	var site, method, cloud string
	var stdin, noStore bool
	login := command("login", "Validar una cuenta y configurar su perfil", func(cmd *cobra.Command, _ []string) error {
		a, err := access()
		if err != nil {
			return err
		}
		name := profile
		if name == "" {
			name = a.Env("JFLOW_PROFILE")
		}
		address := email
		if address == "" {
			address = a.Env("JFLOW_EMAIL")
		}
		p := config.Profile{Provider: "jira-cloud", SiteURL: site, CloudID: cloud, Auth: config.Auth{Method: method, Email: address}}
		// Reuse selected profile settings; explicit flags override stored settings.
		c, err := config.Load(a.Path)
		if err != nil {
			return err
		}
		if name == "" {
			name = c.ActiveProfile
		}
		if old, ok := c.Profiles[name]; ok {
			if !cmd.Flags().Changed("site") {
				p.SiteURL = old.SiteURL
			}
			if !cmd.Flags().Changed("method") {
				p.Auth.Method = old.Auth.Method
			}
			if !cmd.Flags().Changed("cloud-id") {
				p.CloudID = old.CloudID
			}
			if p.Auth.Email == "" {
				p.Auth.Email = old.Auth.Email
			}
		}
		if p.Auth.Email == "" {
			p.Auth.Email = c.Email
		}
		envToken := a.Env("JFLOW_TOKEN")
		interactive := !stdin && !noInput && a.Env("JFLOW_NO_INPUT") != "1" && a.Env("JFLOW_NO_INPUT") != "true"
		inputFile, isFile := cmd.InOrStdin().(*os.File)
		interactive = interactive && isFile && term.IsTerminal(int(inputFile.Fd()))
		if interactive {
			fmt.Fprintln(cmd.ErrOrStderr(), "Crea un token personal: https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/")
			// Read one byte at a time so prompts never buffer token input ahead of ReadPassword.
			prompt := func(label string, value *string) error {
				if *value != "" {
					return nil
				}
				fmt.Fprint(cmd.ErrOrStderr(), label+": ")
				b := []byte{}
				one := make([]byte, 1)
				for len(b) < 4096 {
					n, e := inputFile.Read(one)
					if n > 0 {
						if one[0] == '\n' {
							*value = strings.TrimSpace(string(b))
							return nil
						}
						b = append(b, one[0])
					}
					if e != nil {
						return &domain.Error{Kind: domain.InvalidInput, Message: "Entrada incompleta."}
					}
				}
				return &domain.Error{Kind: domain.InvalidInput, Message: "Entrada demasiado larga."}
			}
			for _, item := range []struct {
				label string
				value *string
			}{{"Perfil", &name}, {"Sitio HTTPS", &p.SiteURL}, {"Correo", &p.Auth.Email}} {
				if err := prompt(item.label, item.value); err != nil {
					return err
				}
			}
			if p.Auth.Method == "api-token-scoped" {
				if err := prompt("Cloud ID real del sitio", &p.CloudID); err != nil {
					return err
				}
			}
		}
		if !config.ValidName(name) {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Indica --profile NOMBRE."}
		}
		if err := p.Validate(); err != nil {
			return err
		}
		var token ports.Secret
		persist := !noStore
		if stdin {
			token, err = readToken(cmd.InOrStdin())
		} else if envToken != "" {
			token = ports.NewSecret(envToken)
			persist = false
		} else if interactive {
			fmt.Fprint(cmd.ErrOrStderr(), "API token (entrada oculta): ")
			var b []byte
			b, err = term.ReadPassword(int(inputFile.Fd()))
			fmt.Fprintln(cmd.ErrOrStderr())
			token = ports.NewSecret(string(b))
			if err != nil {
				err = &domain.Error{Kind: domain.InvalidInput, Message: "No se pudo leer el token."}
			}
		} else {
			return &domain.Error{Kind: domain.Authentication, Message: "Proporciona JFLOW_TOKEN o --token-stdin; no se acepta --token VALOR."}
		}
		if err != nil {
			return err
		}
		result, err := a.Login(cmd.Context(), name, p, token, persist)
		if err != nil {
			return err
		}
		text := fmt.Sprintf("Perfil %s: %s (%s).", result.Profile, result.DisplayName, result.AccountID)
		if !persist {
			text += " Token efímero: vuelve a proporcionarlo en cada invocación."
		}
		return emit(result, text)
	})
	login.Flags().StringVar(&site, "site", "", "URL HTTPS del sitio Jira Cloud")
	login.Flags().StringVar(&method, "method", "api-token-unscoped", "api-token-unscoped o api-token-scoped")
	login.Flags().StringVar(&cloud, "cloud-id", "", "Cloud ID real para tokens con scopes")
	login.Flags().BoolVar(&stdin, "token-stdin", false, "Leer el token desde stdin")
	login.Flags().BoolVar(&noStore, "no-store", false, "Validar sin guardar el token en el llavero")
	auth.AddCommand(login, command("status", "Consultar fuente local de credenciales (sin validación de red)", func(cmd *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		r, e := a.Status(cmd.Context(), profile, email)
		if e != nil {
			return e
		}
		return emit(r, fmt.Sprintf("Perfil %s: credencial disponible en %s; no verificada con Jira.", r.Profile, r.Source))
	}), command("logout", "Eliminar el perfil y su credencial local", func(cmd *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		r, e := a.Logout(cmd.Context(), profile)
		if e != nil {
			return e
		}
		text := "Perfil y credencial local eliminados. El token no se revoca en Atlassian."
		if r["environment_token_present"] == true {
			text += " JFLOW_TOKEN sigue presente en el entorno."
		}
		return emit(r, text)
	}))
	profiles := &cobra.Command{Use: "profile", Short: "Administrar perfiles locales"}
	profiles.AddCommand(command("list", "Listar perfiles y marcar el activo", func(_ *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		r, e := a.Profiles()
		if e != nil {
			return e
		}
		var b strings.Builder
		for _, p := range r {
			mark := " "
			if p.Active {
				mark = "*"
			}
			fmt.Fprintf(&b, "%s %s  %s\n", mark, p.Name, p.SiteURL)
		}
		return emit(r, strings.TrimSuffix(b.String(), "\n"))
	}))
	use := command("use NOMBRE", "Seleccionar el perfil activo", func(cmd *cobra.Command, args []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		if e = a.Use(cmd.Context(), args[0]); e != nil {
			return e
		}
		return emit(map[string]string{"active_profile": args[0]}, "Perfil activo: "+args[0])
	})
	use.Args = cobra.ExactArgs(1)
	profiles.AddCommand(use)
	for _, kind := range []string{"me", "doctor"} {
		var tokenStdin bool
		cmd := command(kind, map[string]string{"me": "Mostrar la identidad autenticada", "doctor": "Comprobar configuración, credencial y conectividad Jira"}[kind], func(cmd *cobra.Command, _ []string) error {
			a, e := access()
			if e != nil {
				return e
			}
			var token ports.Secret
			if tokenStdin {
				token, e = readToken(cmd.InOrStdin())
				if e != nil {
					return e
				}
			}
			r, e := a.Me(cmd.Context(), profile, email, token)
			if e != nil {
				return e
			}
			if cmd.Name() == "doctor" {
				source := "environment"
				if tokenStdin {
					source = "stdin"
				} else if a.Env("JFLOW_TOKEN") == "" {
					source = "keyring"
				}
				tty := false
				if f, ok := cmd.InOrStdin().(*os.File); ok {
					tty = term.IsTerminal(int(f.Fd()))
				}
				return emit(map[string]any{"identity": r, "configuration": "ok", "connectivity": "ok", "authentication": "ok", "credential_source": source, "keyring_checked": source == "keyring", "stdin_tty": tty}, "Configuración, autenticación y conexión Jira correctas. Perfil: "+r.Profile)
			}
			return emit(r, fmt.Sprintf("%s (%s) · perfil %s", r.DisplayName, r.AccountID, r.Profile))
		})
		cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Leer credencial efímera desde stdin")
		root.AddCommand(cmd)
	}
	cfg := &cobra.Command{Use: "config", Short: "Inspeccionar configuración local"}
	cfg.AddCommand(command("path", "Mostrar ruta de configuración", func(_ *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		return emit(map[string]string{"path": a.Path}, a.Path)
	}), command("validate", "Validar configuración sin mostrar credenciales", func(_ *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		_, e = config.Load(a.Path)
		if e != nil {
			return e
		}
		return emit(map[string]bool{"valid": true}, "Configuración válida.")
	}))
	root.AddCommand(auth, profiles, cfg)
}
func readToken(in io.Reader) (ports.Secret, error) {
	b, err := io.ReadAll(io.LimitReader(bufio.NewReader(in), 65537))
	if err != nil || len(b) > 65536 {
		return ports.Secret{}, &domain.Error{Kind: domain.InvalidInput, Message: "No se pudo leer el token o supera 64 KiB."}
	}
	value := strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
	if value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return ports.Secret{}, &domain.Error{Kind: domain.InvalidInput, Message: "El token debe contener una sola línea no vacía."}
	}
	return ports.NewSecret(value), nil
}

// Local commands do not load TLS settings or initialize a network adapter.
type defaultIdentity struct{ CAFile string }

func (d defaultIdentity) Myself(ctx context.Context, p config.Profile, token ports.Secret) (domain.User, error) {
	client, err := jiracloud.New(d.CAFile)
	if err != nil {
		return domain.User{}, err
	}
	return client.Myself(ctx, p, token)
}
