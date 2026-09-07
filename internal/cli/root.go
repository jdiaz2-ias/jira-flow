// Package cli adapts Cobra arguments and streams to application use cases.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
)

// Run owns error rendering and process exit semantics without calling os.Exit.
func Run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, info app.VersionInfo) int {
	return RunWithDependencies(ctx, args, in, out, errOut, info, Dependencies{})
}

func RunWithDependencies(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, info app.VersionInfo, deps Dependencies) int {
	format := requestedFormat(args)
	root := &cobra.Command{
		Use:          "jflow",
		Short:        "Jira Flow: Jira desde tu terminal",
		Long:         "Jira Flow · consultas, perfiles y autenticación Jira Cloud.",
		SilenceUsage: true, SilenceErrors: true,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Selecciona un comando. Consulta jflow --help."}
		},
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetIn(in)
	root.SetOut(out)
	root.SetErr(errOut)
	root.SetArgs(args)
	root.PersistentFlags().StringVar(&format, "format", format, "Formato de salida: plain, table (listados) o json")
	root.PersistentFlags().Bool("no-color", false, "Salida sin colores")
	root.PersistentFlags().Bool("ascii", false, "Salida sin adornos gráficos")
	root.PersistentFlags().Bool("verbose", false, "Diagnóstico básico redactado por stderr")
	root.PersistentPreRunE = func(cmd *cobra.Command, _ []string) error {
		if err := ctx.Err(); err != nil {
			return &domain.Error{Kind: domain.Canceled, Message: "Operación cancelada.", Cause: err}
		}
		if format != "plain" && format != "json" && !(format == "table" && cmd.Annotations["collection"] == "true") {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Formato inválido. Usa plain, json o table para listados."}
		}
		if format == "json" {
			_ = root.PersistentFlags().Set("no-input", "true")
		}
		verbose, _ := root.PersistentFlags().GetBool("verbose")
		if verbose {
			fmt.Fprintln(errOut, "Ejecutando "+cmd.CommandPath())
		}
		return nil
	}
	var writeErr error
	rendered := false
	// Help participates in the JSON contract, including `jflow help version`.
	root.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		if format != "plain" && format != "json" {
			writeErr = &domain.Error{Kind: domain.InvalidInput, Message: "Formato inválido. Usa plain, json o table para listados."}
			return
		}
		var buf bytes.Buffer
		if cmd.Long == "" {
			fmt.Fprintln(&buf, cmd.Short)
		} else {
			fmt.Fprintln(&buf, cmd.Long)
		}
		buf.WriteString(cmd.UsageString())
		if format == "json" {
			writeErr = output.Write(out, output.Success(map[string]string{"help": buf.String()}))
		} else {
			_, writeErr = io.Copy(out, &buf)
		}
	})
	root.AddCommand(&cobra.Command{
		Use: "version", Short: "Mostrar versión, commit y plataforma", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			if format == "json" {
				writeErr = output.Write(out, output.Success(output.VersionData(info)))
			} else {
				_, writeErr = fmt.Fprintf(out, "jflow %s\ncommit: %s\nGo: %s\nplataforma: %s/%s\n", info.Version, info.Commit, info.GoVersion, info.OS, info.Arch)
			}
			return nil
		},
	})
	root.SetHelpCommand(&cobra.Command{
		Use: "help [comando...]", Short: "Mostrar ayuda", Args: cobra.ArbitraryArgs,
		RunE: func(_ *cobra.Command, args []string) error {
			target := root
			if len(args) > 0 {
				var remaining []string
				var err error
				target, remaining, err = root.Find(args)
				if err != nil || len(remaining) != 0 {
					return &domain.Error{Kind: domain.InvalidInput, Message: "Comando desconocido. Consulta jflow --help."}
				}
			}
			return target.Help()
		},
	})
	addAccess(root, deps, func(data any, plain string) error {
		if format == "json" {
			writeErr = output.Write(out, output.Success(data))
		} else {
			_, writeErr = fmt.Fprintln(out, plain)
		}
		return nil
	}, func(envelope output.Envelope, plain string, operationErr error) error {
		if format == "json" {
			writeErr = output.Write(out, envelope)
			rendered = true
		} else if plain != "" {
			_, writeErr = fmt.Fprintln(out, plain)
		}
		return operationErr
	})
	err := root.ExecuteContext(ctx)
	if err != nil {
		// A parser may stop after an earlier --format and before a later one.
		format = requestedFormat(args)
	}
	if writeErr != nil {
		var public *domain.Error
		if errors.As(writeErr, &public) {
			err = writeErr
		} else {
			fmt.Fprintln(errOut, "No se pudo escribir la salida.")
			return 1
		}
	}
	if rendered {
		return domain.ExitCode(err)
	}
	if err == nil {
		return 0
	}
	var public *domain.Error
	if !errors.As(err, &public) {
		// Cobra parse errors can contain arbitrary input; never echo it blindly.
		err = &domain.Error{Kind: domain.InvalidInput, Message: "Argumentos inválidos. Consulta jflow --help.", Cause: err}
	}
	if format == "json" {
		if output.Write(out, output.Failure(err)) != nil {
			fmt.Fprintln(errOut, "No se pudo escribir la salida.")
			return 1
		}
	} else {
		fmt.Fprintln(errOut, err.Error())
	}
	return domain.ExitCode(err)
}

// Resolve output even if Cobra rejects a command before parsing persistent flags.
// Honor --, and the last explicit --format, matching pflag's scalar semantics.
func requestedFormat(args []string) string {
	format := "plain"
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			break
		}
		if strings.HasPrefix(arg, "--format=") {
			format = strings.TrimPrefix(arg, "--format=")
		} else if arg == "--format" && i+1 < len(args) {
			i++
			format = args[i]
		}
	}
	return format
}
