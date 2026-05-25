package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/coterm/coterm/internal/cursor"
	"github.com/coterm/coterm/internal/session"
)

func (app App) read(ctx context.Context, args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("read", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	clientFlag := flags.String("client", "", "client id")
	paneName := flags.String("pane", "", "pane name")
	if err := flags.Parse(args); err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if flags.NArg() != 0 {
		return WriteJSON(stdout, Result{OK: false, Error: "read does not accept positional arguments"})
	}
	if *paneName == "" {
		return WriteJSON(stdout, Result{OK: false, Error: "read requires --pane"})
	}

	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	clientID, err := clientIDFromFlag(*clientFlag)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}

	var result Result
	if err := cursor.WithClientLock(ctx, paths, clientID, func() error {
		clientState, err := cursor.LoadClientState(paths, clientID)
		if err != nil {
			return err
		}
		info, err := session.EnsurePaneContext(ctx, app.tmuxClient(), paths, *paneName)
		if err != nil {
			return err
		}
		captured, err := app.tmuxClient().CapturePane(ctx, info.PaneID)
		if err != nil {
			return fmt.Errorf("capture pane %s: %v", *paneName, err)
		}
		delta, next, external := cursor.Delta(clientState.Cursors[*paneName], captured)
		clientState.Cursors[*paneName] = next
		if err := cursor.SaveClientState(paths, clientState); err != nil {
			return err
		}

		result = Result{
			OK:                      true,
			ClientID:                clientState.ClientID,
			Pane:                    *paneName,
			OutputDelta:             delta,
			ExternalChangesDetected: external,
		}
		return nil
	}); err != nil {
		return WriteJSON(stdout, Result{OK: false, ClientID: clientID, Pane: *paneName, Error: err.Error()})
	}

	return WriteJSON(stdout, result)
}
