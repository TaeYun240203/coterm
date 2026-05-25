package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type failingWriter struct{}

func (failingWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestHelpDoesNotExposeHiddenCommands(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main(context.Background(), []string{"help"}, nil, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code = %d, stderr = %s", code, stderr.String())
	}
	out := stdout.String()
	if strings.Contains(out, "full-access") {
		t.Fatalf("help exposed hidden full-access command: %s", out)
	}
	if !strings.Contains(out, "coterm run") {
		t.Fatalf("help did not include run command: %s", out)
	}
}

func TestUnknownCommandReturnsJSONError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Main(context.Background(), []string{"wat"}, nil, &stdout, &stderr)
	if code == 0 {
		t.Fatal("expected non-zero exit code")
	}
	var result Result
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("expected JSON error on stdout, got %q stderr %q", stdout.String(), stderr.String())
	}
	if result.OK {
		t.Fatalf("expected OK false, got %+v", result)
	}
	if !strings.Contains(result.Error, "unknown command") {
		t.Fatalf("expected unknown command error, got %+v", result)
	}
}

func TestWriteJSONReturnsFailureWhenWriteFails(t *testing.T) {
	code := WriteJSON(failingWriter{}, Result{OK: true})
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
}
