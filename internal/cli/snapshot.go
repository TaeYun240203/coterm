package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/coterm/coterm/internal/cursor"
	"github.com/coterm/coterm/internal/session"
)

const (
	defaultSnapshotLines = 200
	defaultSnapshotBytes = 64 * 1024
)

func (app App) snapshot(ctx context.Context, args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	clientFlag := flags.String("client", "", "client id")
	paneName := flags.String("pane", "", "pane name")
	lines := flags.Int("lines", defaultSnapshotLines, "maximum lines")
	bytes := flags.Int("bytes", defaultSnapshotBytes, "maximum bytes")
	if err := flags.Parse(args); err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if flags.NArg() != 0 {
		return WriteJSON(stdout, Result{OK: false, Error: "snapshot does not accept positional arguments"})
	}
	if *paneName == "" {
		return WriteJSON(stdout, Result{OK: false, Error: "snapshot requires --pane"})
	}
	if *lines < 0 {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: "--lines must be non-negative"})
	}
	if *bytes < 0 {
		return WriteJSON(stdout, Result{OK: false, Pane: *paneName, Error: "--bytes must be non-negative"})
	}

	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	clientState, err := loadClientState(paths, *clientFlag)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	info, err := session.EnsurePaneContext(ctx, app.tmuxClient(), paths, *paneName)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, ClientID: clientState.ClientID, Pane: *paneName, Error: err.Error()})
	}
	captured, err := app.tmuxClient().CapturePane(ctx, info.PaneID)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, ClientID: clientState.ClientID, Pane: *paneName, Error: fmt.Sprintf("capture pane %s: %v", *paneName, err)})
	}
	output, truncated := cursor.Tail(captured, *lines, *bytes)
	clientState.Cursors[*paneName] = cursor.Cursor{LineCount: len(cursor.SplitLines(captured))}
	if err := cursor.SaveClientState(paths, clientState); err != nil {
		return WriteJSON(stdout, Result{OK: false, ClientID: clientState.ClientID, Pane: *paneName, Error: err.Error()})
	}

	result := Result{
		OK:          true,
		ClientID:    clientState.ClientID,
		Pane:        *paneName,
		OutputDelta: output,
		Truncated:   truncated,
	}
	if truncated {
		result.TruncatedFrom = "head"
	}
	return WriteJSON(stdout, result)
}
