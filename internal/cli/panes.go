package cli

import (
	"context"
	"io"

	"github.com/coterm/coterm/internal/session"
)

func (app App) panes(ctx context.Context, stdout io.Writer) int {
	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	panes, err := session.ListMappedPanesContext(ctx, app.tmuxClient(), paths)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}

	return WriteJSON(stdout, Result{OK: true, Panes: panes})
}
