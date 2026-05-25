package cli

import (
	"io"

	"github.com/coterm/coterm/internal/state"
)

func (app App) fullAccess(args []string, stdout io.Writer) int {
	if len(args) != 1 {
		return WriteJSON(stdout, Result{OK: false, Error: "usage: coterm full-access on|off|status"})
	}
	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	config, err := state.LoadConfig(paths)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	switch args[0] {
	case "on":
		config.FullAccess = true
		if err := state.SaveConfig(paths, config); err != nil {
			return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
		}
	case "off":
		config.FullAccess = false
		if err := state.SaveConfig(paths, config); err != nil {
			return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
		}
	case "status":
	default:
		return WriteJSON(stdout, Result{OK: false, Error: "usage: coterm full-access on|off|status"})
	}
	fullAccess := config.FullAccess
	return WriteJSON(stdout, Result{OK: true, FullAccess: &fullAccess})
}
