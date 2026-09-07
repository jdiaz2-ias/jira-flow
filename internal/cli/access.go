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
	"jira-flow.local/jflow/internal/cache"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
	"jira-flow.local/jflow/internal/ports"
	"jira-flow.local/jflow/internal/provider/jiracloud"
	"jira-flow.local/jflow/internal/secretstore"
)

// Dependencies allow command tests to exercise full flows without a real keyring.
type Dependencies struct {
	IssueReader func(config.Profile, ports.Secret) (ports.IssueReader, error)
	Cache       *cache.Memory
	Browser     ports.Browser
	Env         func(string) string
	Secrets     ports.SecretStore
	Jira        app.IdentityClient
}

func addAccess(root *cobra.Command, deps Dependencies, emit func(any, string) error, emitRead func(output.Envelope, string, error) error) {
	var profile, email string
	var noInput bool
	root.PersistentFlags().StringVar(&profile, "profile", "", "Profile for this invocation")
	root.PersistentFlags().StringVar(&email, "email", "", "Atlassian email for this invocation")
	root.PersistentFlags().BoolVar(&noInput, "no-input", false, "Do not request interactive input")
	access := func() (app.Access, error) {
		env := deps.Env
		if env == nil {
			env = os.Getenv
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return app.Access{}, &domain.Error{Kind: domain.InvalidInput, Message: "Could not resolve home directory."}
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
	auth := &cobra.Command{Use: "auth", Short: "Personal authentication with Jira Cloud"}
	var site, method, cloud string
	var stdin, noStore bool
	login := command("login", "Validate an account and set up its profile", func(cmd *cobra.Command, _ []string) error {
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
			fmt.Fprintln(cmd.ErrOrStderr(), "Create a personal token: https://support.atlassian.com/atlassian-account/docs/manage-api-tokens-for-your-atlassian-account/")
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
						return &domain.Error{Kind: domain.InvalidInput, Message: "Incomplete input."}
					}
				}
				return &domain.Error{Kind: domain.InvalidInput, Message: "Input too long."}
			}
			for _, item := range []struct {
				label string
				value *string
			}{{"Profile", &name}, {"HTTPS Site", &p.SiteURL}, {"Email", &p.Auth.Email}} {
				if err := prompt(item.label, item.value); err != nil {
					return err
				}
			}
			if p.Auth.Method == "api-token-scoped" {
				if err := prompt("Real Cloud ID of the site", &p.CloudID); err != nil {
					return err
				}
			}
		}
		if !config.ValidName(name) {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Provide --profile NAME."}
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
			fmt.Fprint(cmd.ErrOrStderr(), "API token (hidden input): ")
			var b []byte
			b, err = term.ReadPassword(int(inputFile.Fd()))
			fmt.Fprintln(cmd.ErrOrStderr())
			token = ports.NewSecret(string(b))
			if err != nil {
				err = &domain.Error{Kind: domain.InvalidInput, Message: "Could not read token."}
			}
		} else {
			return &domain.Error{Kind: domain.Authentication, Message: "Provide JFLOW_TOKEN or --token-stdin; --token VALUE is not accepted."}
		}
		if err != nil {
			return err
		}
		result, err := a.Login(cmd.Context(), name, p, token, persist)
		if err != nil {
			return err
		}
		text := fmt.Sprintf("Profile %s: %s (%s).", result.Profile, result.DisplayName, result.AccountID)
		if !persist {
			text += " Ephemeral token: provide it again on every invocation."
		}
		return emit(result, text)
	})
	login.Flags().StringVar(&site, "site", "", "HTTPS URL of the Jira Cloud site")
	login.Flags().StringVar(&method, "method", "api-token-unscoped", "api-token-unscoped or api-token-scoped")
	login.Flags().StringVar(&cloud, "cloud-id", "", "Real Cloud ID for scoped tokens")
	login.Flags().BoolVar(&stdin, "token-stdin", false, "Read token from stdin")
	login.Flags().BoolVar(&noStore, "no-store", false, "Validate without saving token in keyring")
	auth.AddCommand(login, command("status", "Check local credential source (no network validation)", func(cmd *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		r, e := a.Status(cmd.Context(), profile, email)
		if e != nil {
			return e
		}
		return emit(r, fmt.Sprintf("Profile %s: credential available in %s; not verified with Jira.", r.Profile, r.Source))
	}), command("logout", "Delete the profile and its local credential", func(cmd *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		r, e := a.Logout(cmd.Context(), profile)
		if e != nil {
			return e
		}
		text := "Profile and local credential deleted. The token is not revoked in Atlassian."
		if r["environment_token_present"] == true {
			text += " JFLOW_TOKEN is still present in the environment."
		}
		return emit(r, text)
	}))
	profiles := &cobra.Command{Use: "profile", Short: "Manage local profiles"}
	profiles.AddCommand(command("list", "List profiles and mark the active one", func(_ *cobra.Command, _ []string) error {
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
	use := command("use NOMBRE", "Select the active profile", func(cmd *cobra.Command, args []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		if e = a.Use(cmd.Context(), args[0]); e != nil {
			return e
		}
		return emit(map[string]string{"active_profile": args[0]}, "Active profile: "+args[0])
	})
	use.Args = cobra.ExactArgs(1)
	profiles.AddCommand(use)
	for _, kind := range []string{"me", "doctor"} {
		var tokenStdin bool
		cmd := command(kind, map[string]string{"me": "Show the authenticated identity", "doctor": "Check configuration, credential, and Jira connectivity"}[kind], func(cmd *cobra.Command, _ []string) error {
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
				return emit(map[string]any{"identity": r, "configuration": "ok", "connectivity": "ok", "authentication": "ok", "credential_source": source, "keyring_checked": source == "keyring", "stdin_tty": tty}, "Configuration, authentication, and Jira connection OK. Profile: "+r.Profile)
			}
			return emit(r, fmt.Sprintf("%s (%s) · profile %s", r.DisplayName, r.AccountID, r.Profile))
		})
		cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "Read ephemeral credential from stdin")
		root.AddCommand(cmd)
	}
	cfg := &cobra.Command{Use: "config", Short: "Inspect local configuration"}
	cfg.AddCommand(command("path", "Show configuration path", func(_ *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		return emit(map[string]string{"path": a.Path}, a.Path)
	}), command("validate", "Validate configuration without showing credentials", func(_ *cobra.Command, _ []string) error {
		a, e := access()
		if e != nil {
			return e
		}
		_, e = config.Load(a.Path)
		if e != nil {
			return e
		}
		return emit(map[string]bool{"valid": true}, "Configuration valid.")
	}))
	root.AddCommand(auth, profiles, cfg)
	addReading(root, deps, access, &profile, &email, emitRead)
}

func readToken(in io.Reader) (ports.Secret, error) {
	b, err := io.ReadAll(io.LimitReader(bufio.NewReader(in), 65537))
	if err != nil || len(b) > 65536 {
		return ports.Secret{}, &domain.Error{Kind: domain.InvalidInput, Message: "Could not read token or it exceeds 64 KiB."}
	}
	value := strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r")
	if value == "" || strings.ContainsAny(value, "\r\n\x00") {
		return ports.Secret{}, &domain.Error{Kind: domain.InvalidInput, Message: "Token must be a single non-empty line."}
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
