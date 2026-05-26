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
	if !result.OK || result.FullAccess == nil || !*result.FullAccess {
		t.Fatalf("status result = %+v", result)
	}
}

func TestFullAccessStatusIncludesFalseWhenOff(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"full-access", "off"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	assertJSONField(t, stdout.Bytes(), "full_access", false)

	stdout.Reset()
	code = app.Main(context.Background(), []string{"full-access", "status"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	assertJSONField(t, stdout.Bytes(), "full_access", false)
}

func TestNonFullAccessResponsesOmitFullAccess(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"panes"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	assertJSONFieldAbsent(t, stdout.Bytes(), "full_access")

	stdout.Reset()
	code = app.Main(context.Background(), []string{"unknown-command"}, nil, &stdout, io.Discard)
	if code == 0 {
		t.Fatalf("code = %d output = %s, want failure", code, stdout.String())
	}
	assertJSONFieldAbsent(t, stdout.Bytes(), "full_access")
}

func assertJSONField(t *testing.T, data []byte, key string, want any) {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", string(data), err)
	}
	value, ok := raw[key]
	if !ok {
		t.Fatalf("output missing %s: %s", key, string(data))
	}
	if value != want {
		t.Fatalf("%s = %v, want %v", key, value, want)
	}
}

func assertJSONFieldAbsent(t *testing.T, data []byte, key string) {
	t.Helper()
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("expected JSON result, got %q: %v", string(data), err)
	}
	if _, ok := raw[key]; ok {
		t.Fatalf("output includes %s: %s", key, string(data))
	}
}
