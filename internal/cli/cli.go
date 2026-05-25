package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/coterm/coterm/internal/tmux"
)

type App struct {
	Tmux        tmux.Client
	Getwd       func() (string, error)
	UserHomeDir func() (string, error)
}

func Main(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	app := App{
		Tmux:        tmux.NewExec(),
		Getwd:       os.Getwd,
		UserHomeDir: os.UserHomeDir,
	}
	return app.Main(ctx, args, stdin, stdout, stderr)
}

func (app App) Main(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = stderr

	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, HelpText)
		return 0
	}
	switch args[0] {
	case "open":
		return app.open(ctx, stdout)
	case "panes":
		return app.panes(ctx, stdout)
	case "sync":
		return app.sync(ctx, args[1:], stdout)
	case "read":
		return app.read(ctx, args[1:], stdout)
	case "snapshot":
		return app.snapshot(ctx, args[1:], stdout)
	case "run":
		return app.run(ctx, args[1:], stdin, stdout)
	case "pane":
		return app.pane(ctx, args[1:], stdout)
	case "export":
		return app.export(args[1:], stdout)
	case "uninstall":
		return app.uninstall(args[1:], stdout)
	case "debug":
		return app.debug(ctx, stdout)
	case "full-access":
		return app.fullAccess(args[1:], stdout)
	default:
		return WriteJSON(stdout, Result{OK: false, Error: "unknown command: " + args[0]})
	}
}

func (app App) tmuxClient() tmux.Client {
	if app.Tmux != nil {
		return app.Tmux
	}
	return tmux.NewExec()
}

func (app App) cwd() (string, error) {
	if app.Getwd != nil {
		return app.Getwd()
	}
	return os.Getwd()
}

func (app App) homeDir() (string, error) {
	if app.UserHomeDir != nil {
		return app.UserHomeDir()
	}
	return os.UserHomeDir()
}
