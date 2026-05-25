package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/coterm/coterm/internal/cursor"
	"github.com/coterm/coterm/internal/session"
	"github.com/coterm/coterm/internal/state"
)

func (app App) sync(ctx context.Context, args []string, stdout io.Writer) int {
	flags := flag.NewFlagSet("sync", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	clientFlag := flags.String("client", "", "client id")
	if err := flags.Parse(args); err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	if flags.NArg() != 0 {
		return WriteJSON(stdout, Result{OK: false, Error: "sync does not accept positional arguments"})
	}

	paths, err := app.workspaceStatePaths()
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	clientState, err := loadClientState(paths, *clientFlag)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}
	panes, err := session.ListMappedPanesContext(ctx, app.tmuxClient(), paths)
	if err != nil {
		return WriteJSON(stdout, Result{OK: false, Error: err.Error()})
	}

	names := sortedPaneNames(panes)
	changed := make([]string, 0, len(names))
	deltas := make(map[string]string)
	external := false
	for _, name := range names {
		if isInternalPaneName(name) {
			continue
		}
		captured, err := app.tmuxClient().CapturePane(ctx, panes[name])
		if err != nil {
			return WriteJSON(stdout, Result{OK: false, ClientID: clientState.ClientID, Error: fmt.Sprintf("capture pane %s: %v", name, err)})
		}
		delta, next, paneExternal := cursor.Delta(clientState.Cursors[name], captured)
		clientState.Cursors[name] = next
		if paneExternal {
			external = true
		}
		if delta != "" {
			changed = append(changed, name)
			deltas[name] = delta
		}
	}
	if err := cursor.SaveClientState(paths, clientState); err != nil {
		return WriteJSON(stdout, Result{OK: false, ClientID: clientState.ClientID, Error: err.Error()})
	}

	return WriteJSON(stdout, Result{
		OK:                      true,
		ClientID:                clientState.ClientID,
		ChangedPanes:            changed,
		OutputDeltas:            deltas,
		ExternalChangesDetected: external,
	})
}

func loadClientState(paths state.Paths, clientID string) (cursor.ClientState, error) {
	if clientID == "" {
		var err error
		clientID, err = cursor.NewClientID()
		if err != nil {
			return cursor.ClientState{}, err
		}
	}
	return cursor.LoadClientState(paths, clientID)
}

func sortedPaneNames(panes map[string]string) []string {
	names := make([]string, 0, len(panes))
	for name := range panes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isInternalPaneName(name string) bool {
	return name == "permission" || name == "permission1" || strings.HasPrefix(name, "permission-")
}
