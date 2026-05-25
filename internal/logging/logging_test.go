package logging

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	input := "OPENAI_API_KEY=sk-abc123\npassword = hunter2\nDATABASE_URL=postgres://secret\nAWS_ACCESS_KEY_ID=AKIA_TEST\n-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"
	got := Redact(input)
	for _, secret := range []string{"sk-abc123", "hunter2", "postgres://secret", "AKIA_TEST", "abc"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
}

func TestRedactJSONSecrets(t *testing.T) {
	input := `{"api_key":"sk-json","database_url":"postgres://secret","session_id":"session-visible","command_id":"command-visible","nested":{"token":"tok-json"},"items":[{"password":"pw-json"}],"output_delta":"-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----","safe":"ok"}`
	got := Redact(input)
	for _, secret := range []string{"sk-json", "tok-json", "pw-json", "postgres://secret", "abc"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
	for _, want := range []string{`"api_key":"[REDACTED]"`, `"database_url":"[REDACTED]"`, `"session_id":"session-visible"`, `"command_id":"command-visible"`, `"token":"[REDACTED]"`, `"password":"[REDACTED]"`, `"output_delta":"[REDACTED PRIVATE KEY]"`, `"safe":"ok"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("redacted JSON missing %q: %s", want, got)
		}
	}
}

func TestAppendCommandLogWritesRedactedJSONL(t *testing.T) {
	path := filepath.Join(t.TempDir(), "commands.jsonl")
	code := 0
	err := AppendCommandLog(path, CommandLog{
		Event:       "completed",
		CommandID:   "cmd_123",
		ClientID:    "cl_123",
		Pane:        "main1",
		Argv:        []string{"sh", "-c", "echo OPENAI_API_KEY=sk-abc123"},
		ExitCode:    &code,
		OutputDelta: "password = hunter2\n",
	})
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(data)
	if !strings.HasSuffix(got, "\n") {
		t.Fatalf("log entry is not newline terminated: %q", got)
	}
	for _, secret := range []string{"sk-abc123", "hunter2"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
	for _, want := range []string{`"event":"completed"`, `"command_id":"cmd_123"`, `"pane":"main1"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("log missing %q: %s", want, got)
		}
	}
}
