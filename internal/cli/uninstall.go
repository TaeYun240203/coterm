package cli

import (
	"flag"
	"io"

	"github.com/coterm/coterm/internal/install"
)

type UninstallResult struct {
	OK      bool     `json:"ok"`
	Removed []string `json:"removed,omitempty"`
	Skipped []string `json:"skipped,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func (app App) uninstall(args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	purge := flags.Bool("purge", false, "remove global coterm cache/state")
	if err := flags.Parse(args); err != nil {
		return writeUninstallJSON(stdout, UninstallResult{OK: false, Error: err.Error()})
	}
	if flags.NArg() != 0 {
		return writeUninstallJSON(stdout, UninstallResult{OK: false, Error: "uninstall does not accept positional arguments"})
	}
	home, err := app.homeDir()
	if err != nil {
		return writeUninstallJSON(stdout, UninstallResult{OK: false, Error: err.Error()})
	}
	result, err := install.Uninstall(install.Options{Home: home, Purge: *purge})
	if err != nil {
		return writeUninstallJSON(stdout, UninstallResult{OK: false, Error: err.Error()})
	}
	return writeUninstallJSON(stdout, UninstallResult{OK: true, Removed: result.Removed, Skipped: result.Skipped})
}

func writeUninstallJSON(stdout io.Writer, result UninstallResult) int {
	if result.OK {
		result.Error = ""
	}
	return writeSimpleJSON(stdout, result, result.OK)
}
