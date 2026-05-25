package cli

import (
	"context"
	"fmt"
	"io"
)

func Main(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, HelpText)
		return 0
	}
	switch args[0] {
	case "open", "run", "sync", "read", "snapshot", "panes", "pane", "export", "uninstall", "debug":
		return WriteJSON(stdout, Result{OK: false, Error: "command not implemented yet"})
	case "full-access":
		return WriteJSON(stdout, Result{OK: false, Error: "command not implemented yet"})
	default:
		return WriteJSON(stdout, Result{OK: false, Error: "unknown command: " + args[0]})
	}
}
