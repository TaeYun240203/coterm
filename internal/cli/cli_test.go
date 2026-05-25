package cli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

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
	if !strings.Contains(stdout.String(), `"ok":false`) {
		t.Fatalf("expected JSON error on stdout, got %q stderr %q", stdout.String(), stderr.String())
	}
}
