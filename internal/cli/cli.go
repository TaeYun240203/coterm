package cli

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/coterm/coterm/internal/tmux"
)

type App struct {
	Tmux  tmux.Client
	Getwd func() (string, error)
}

func Main(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	app := App{
		Tmux:  tmux.NewExec(),
		Getwd: os.Getwd,
	}
	return app.Main(ctx, args, stdin, stdout, stderr)
}

func (app App) Main(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	_ = stdin
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
	case "run", "pane", "export", "uninstall", "debug":
		return WriteJSON(stdout, Result{OK: false, Error: "command not implemented yet"})
	case "full-access":
		return WriteJSON(stdout, Result{OK: false, Error: "command not implemented yet"})
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
