package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/cli"
)

// Set by the build; go run keeps these development defaults.
var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := cli.Run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, app.BuildInfo(version, commit))
	stop()
	os.Exit(code)
}
