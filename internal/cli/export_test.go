package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportRejectsNonJSONLFormat(t *testing.T) {
	app, _, _ := NewTestApp(t)

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"export", "--format", "json"}, nil, &stdout, io.Discard)
	if code == 0 {
		t.Fatal("expected non-zero code")
	}
	if !strings.Contains(stdout.String(), `"ok":false`) {
		t.Fatalf("expected JSON error, got %s", stdout.String())
	}
}

func TestExportStreamsLogsInLexicalOrderWithRedaction(t *testing.T) {
	app, _, workspace := NewTestApp(t)
	logDir := filepath.Join(workspace, ".coterm", "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "b.jsonl"), []byte(`{"msg":"password=hunter2"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "a.jsonl"), []byte("OPENAI_API_KEY=sk-test\n-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----\n"+`{"api_key":"sk-json","token":"tok-json"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"export", "--format", "jsonl"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	got := stdout.String()
	if strings.Contains(got, `"ok":true`) {
		t.Fatalf("successful export was wrapped in Result JSON: %s", got)
	}
	first := strings.Index(got, "OPENAI_API_KEY")
	second := strings.Index(got, `"msg"`)
	if first < 0 || second < 0 || first > second {
		t.Fatalf("logs were not streamed in lexical order: %s", got)
	}
	for _, secret := range []string{"sk-test", "hunter2", "abc", "sk-json", "tok-json"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in export: %s", secret, got)
		}
	}
	if strings.Contains(got, "BEGIN OPENSSH PRIVATE KEY") || strings.Contains(got, "END OPENSSH PRIVATE KEY") {
		t.Fatalf("private key block marker leaked in export: %s", got)
	}
}
