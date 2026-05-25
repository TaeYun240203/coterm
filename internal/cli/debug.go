package cli

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	debugpkg "github.com/coterm/coterm/internal/debug"
	"github.com/coterm/coterm/internal/state"
	"github.com/coterm/coterm/internal/workspace"
)

const Version = "0.1.0"

func (app App) debug(ctx context.Context, args []string, stdout io.Writer) int {
	if len(args) != 0 {
		return WriteJSON(stdout, Result{OK: false, Error: "debug does not accept positional arguments"})
	}
	paths, err := app.workspaceStatePathsNoEnsure()
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

func (app App) workspaceStatePathsNoEnsure() (state.Paths, error) {
	root, err := app.cwd()
	if err != nil {
		return state.Paths{}, err
	}
	workspaceRoot, err := workspace.FindRoot(root)
	if err != nil {
		return state.Paths{}, err
	}
	dir := filepath.Join(workspaceRoot, ".coterm")
	return state.Paths{
		Workspace:      workspaceRoot,
		Dir:            dir,
		SessionFile:    filepath.Join(dir, "session.toml"),
		PanesFile:      filepath.Join(dir, "panes.toml"),
		ConfigFile:     filepath.Join(dir, "config.toml"),
		ClientsDir:     filepath.Join(dir, "clients"),
		LogsDir:        filepath.Join(dir, "logs"),
		CommandsDir:    filepath.Join(dir, "cache", "commands"),
		PermissionsDir: filepath.Join(dir, "cache", "permissions"),
	}, nil
}
