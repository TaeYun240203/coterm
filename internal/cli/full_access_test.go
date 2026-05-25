package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"testing"

	"github.com/coterm/coterm/internal/state"
)

func TestFullAccessHiddenCommandPersistsWorkspaceConfig(t *testing.T) {
	app, _, workspace := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"full-access", "on"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	paths, err := state.Ensure(workspace)
	if err != nil {
		t.Fatal(err)
	}
	config, err := state.LoadConfig(paths)
	if err != nil {
		t.Fatal(err)
	}
	if !config.FullAccess {
		t.Fatalf("full_access = false, want true")
	}

	stdout.Reset()
	code = app.Main(context.Background(), []string{"full-access", "status"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", stdout.String(), err)
	}
	if !result.OK || !result.FullAccess {
		t.Fatalf("status result = %+v", result)
	}
}
