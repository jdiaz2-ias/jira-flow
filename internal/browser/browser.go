// Package browser launches the platform URL opener without invoking a shell.
package browser

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"jira-flow.local/jflow/internal/domain"
)

type Launcher struct {
	OS  string
	Env func(string) string
	Run func(context.Context, string, ...string) error
}

func New() *Launcher {
	return &Launcher{OS: runtime.GOOS, Env: os.Getenv, Run: func(ctx context.Context, name string, args ...string) error {
		cmd := exec.CommandContext(ctx, name, args...)
		cmd.WaitDelay = time.Second
		return cmd.Run()
	}}
}
func (b *Launcher) Open(ctx context.Context, link string) error {
	u, err := url.Parse(link)
	if err != nil || u.Scheme != "https" || u.User != nil || !strings.HasSuffix(u.Hostname(), ".atlassian.net") || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(u.Path, "/browse/") {
		return &domain.Error{Kind: domain.InvalidInput, Message: "URL de issue inválida."}
	}
	if _, err := domain.IssueKey(strings.TrimPrefix(u.Path, "/browse/")); err != nil {
		return err
	}

	if b.OS == "linux" && b.Env("DISPLAY") == "" && b.Env("WAYLAND_DISPLAY") == "" {
		return &domain.Error{Kind: domain.Unsupported, Message: "No hay sesión gráfica; abre la URL en tu navegador: " + link}
	}
	program := ""
	switch b.OS {
	case "darwin":
		program = "/usr/bin/open"
	case "linux":
		program = "xdg-open"
	default:
		return &domain.Error{Kind: domain.Unsupported, Message: "No hay lanzador compatible; abre la URL: " + link}
	}
	// Link was constructed from the validated profile and issue key by the use case.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := b.Run(ctx, program, link); err != nil {
		if ctx.Err() != nil {
			return &domain.Error{Kind: domain.Canceled, Message: "Apertura cancelada o plazo agotado. URL: " + link}
		}
		return &domain.Error{Kind: domain.Unsupported, Message: "No se pudo iniciar el navegador; abre la URL: " + link}
	}
	return nil
}
