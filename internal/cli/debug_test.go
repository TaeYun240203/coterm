package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDebugCLIReturnsOKJSONWithoutHiddenStrings(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"debug"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	var body map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &body); err != nil {
		t.Fatalf("debug did not return JSON: %v: %s", err, stdout.String())
	}
	if body["ok"] != true {
		t.Fatalf("debug ok = %v body = %+v", body["ok"], body)
	}
	out := stdout.String()
	for _, forbidden := range []string{"full-access", "permission1", "tmux kill-pane"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("debug output exposed %q: %s", forbidden, out)
		}
	}
}

func TestDebugCLIDoesNotCreateCotermStateOrGitignore(t *testing.T) {
	app, _, workspace := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"debug"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	if _, err := os.Stat(filepath.Join(workspace, ".coterm")); !os.IsNotExist(err) {
		t.Fatalf("debug created .coterm, err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, ".gitignore")); !os.IsNotExist(err) {
		t.Fatalf("debug created .gitignore, err = %v", err)
	}
}

func TestDebugCLIRejectsPositionalArgs(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"debug", "extra"}, nil, &stdout, io.Discard)
	if code == 0 {
		t.Fatalf("code = %d output = %s, want failure", code, stdout.String())
	}
	assertJSONErrorContains(t, stdout.Bytes(), "debug does not accept")
}
