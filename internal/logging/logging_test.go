package logging

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactSecrets(t *testing.T) {
	input := "OPENAI_API_KEY=sk-abc123\npassword = hunter2\nDATABASE_URL=postgres://secret\nAWS_ACCESS_KEY_ID=AKIA_TEST\nAuthorization: Bearer ghp_secret\n-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----"
	got := Redact(input)
	for _, secret := range []string{"sk-abc123", "hunter2", "postgres://secret", "AKIA_TEST", "ghp_secret", "abc"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
}

func TestRedactJSONSecrets(t *testing.T) {
	input := `{"api_key":"sk-json","database_url":"postgres://secret","authorization":"Bearer auth-json","session_id":"session-visible","command_id":"command-visible","nested":{"token":"tok-json"},"items":[{"password":"pw-json"}],"output_delta":"Authorization: Bearer ghp_json\n-----BEGIN OPENSSH PRIVATE KEY-----\nabc\n-----END OPENSSH PRIVATE KEY-----","safe":"ok"}`
	got := Redact(input)
	for _, secret := range []string{"sk-json", "auth-json", "tok-json", "pw-json", "postgres://secret", "ghp_json", "abc"} {
		if strings.Contains(got, secret) {
			t.Fatalf("secret %q leaked in %q", secret, got)
		}
	}
	for _, want := range []string{`"api_key":"[REDACTED]"`, `"database_url":"[REDACTED]"`, `"authorization":"[REDACTED]"`, `"session_id":"session-visible"`, `"command_id":"command-visible"`, `"token":"[REDACTED]"`, `"password":"[REDACTED]"`, `"output_delta":"Authorization: [REDACTED]\n[REDACTED PRIVATE KEY]"`, `"safe":"ok"`} {
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

func TestRedactWriterHandlesLargeJSONLine(t *testing.T) {
	var input bytes.Buffer
	input.WriteString(`{"output_delta":"`)
	input.WriteString(strings.Repeat("x", 11*1024*1024))
	input.WriteString(`","token":"tok-large"}`)
	input.WriteByte('\n')

	var output bytes.Buffer
	if err := RedactWriter(&output, &input); err != nil {
		t.Fatal(err)
	}
	got := output.String()
	if strings.Contains(got, "tok-large") {
		t.Fatalf("secret leaked in large JSONL line")
	}
	if !strings.Contains(got, `"token":"[REDACTED]"`) {
		t.Fatalf("large JSONL line was not redacted")
	}
}

func TestRedactWriterRejectsInvalidJSONL(t *testing.T) {
	var output bytes.Buffer
	err := RedactWriter(&output, strings.NewReader(`{"msg":"ok"}`+"\nnot json\n"))
	if err == nil {
		t.Fatal("expected invalid JSONL error")
	}
	if !strings.Contains(err.Error(), "invalid JSONL log line 2") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output.String(), `"msg":"ok"`) {
		t.Fatalf("expected first valid record before invalid line: %s", output.String())
	}
}

func TestRedactWriterAllowsLastLineWithoutNewline(t *testing.T) {
	var output bytes.Buffer
	if err := RedactWriter(&output, strings.NewReader(`{"password":"pw"}`)); err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(&output)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(got), "pw") || !strings.Contains(string(got), `"password":"[REDACTED]"`) {
		t.Fatalf("unexpected output: %s", string(got))
	}
}
