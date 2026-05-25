package cli

import (
	"context"
	"fmt"
	"io"

	debugpkg "github.com/coterm/coterm/internal/debug"
)

const Version = "0.1.0"

func (app App) debug(ctx context.Context, stdout io.Writer) int {
	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	report, err := debugpkg.Build(ctx, debugpkg.Options{
		Version:   Version,
		Workspace: paths.Workspace,
		Paths:     paths,
		Tmux:      app.tmuxClient(),
	})
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if _, err := fmt.Fprintln(stdout, report.JSON()); err != nil {
		return 1
	}
	return 0
}
