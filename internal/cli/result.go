package cli

import (
	"encoding/json"
	"fmt"
	"io"
)

type Result struct {
	OK                      bool              `json:"ok"`
	ClientID                string            `json:"client_id,omitempty"`
	Pane                    string            `json:"pane,omitempty"`
	CommandID               string            `json:"command_id,omitempty"`
	ExitCode                *int              `json:"exit_code,omitempty"`
	OutputDelta             string            `json:"output_delta,omitempty"`
	OutputDeltas            map[string]string `json:"output_deltas,omitempty"`
	Panes                   map[string]string `json:"panes,omitempty"`
	ChangedPanes            []string          `json:"changed_panes,omitempty"`
	Truncated               bool              `json:"truncated"`
	TruncatedFrom           string            `json:"truncated_from,omitempty"`
	NextCursor              string            `json:"next_cursor,omitempty"`
	ExternalChangesDetected bool              `json:"external_changes_detected"`
	PermissionRequired      bool              `json:"permission_required"`
	PermissionDenied        bool              `json:"permission_denied"`
	PermissionTimedOut      bool              `json:"permission_timed_out"`
	FullAccess              *bool             `json:"full_access,omitempty"`
	OpenCommand             string            `json:"open_command,omitempty"`
	Error                   string            `json:"error,omitempty"`
}

func WriteJSON(w io.Writer, result Result) int {
	if result.OK {
		result.Error = ""
	}
	return writeSimpleJSON(w, result, result.OK)
}

func writeSimpleJSON(w io.Writer, value any, ok bool) int {
	data, err := json.Marshal(value)
	if err != nil {
		fmt.Fprintf(w, `{"ok":false,"error":%q}`+"\n", err.Error())
		return 1
	}
	if _, err := fmt.Fprintln(w, string(data)); err != nil {
		return 1
	}
	if ok {
		return 0
	}
	return 1
}
