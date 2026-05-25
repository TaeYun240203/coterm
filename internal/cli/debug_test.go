package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
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
