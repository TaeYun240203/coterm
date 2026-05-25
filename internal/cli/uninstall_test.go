package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallCLIUsesInjectedHome(t *testing.T) {
	app, _, _ := NewTestApp(t)
	home := filepath.Join(t.TempDir(), "home")
	app.UserHomeDir = func() (string, error) { return home, nil }
	binary := filepath.Join(home, ".local", "bin", "coterm")
	skill := filepath.Join(home, ".codex", "skills", "coterm-shared-terminal")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(skill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binary, []byte("bin"), 0o755); err != nil {
		t.Fatal(err)
	}

	var stdout bytes.Buffer
	code := app.Main(context.Background(), []string{"uninstall"}, nil, &stdout, io.Discard)
	if code != 0 {
		t.Fatalf("code = %d output = %s", code, stdout.String())
	}
	var result UninstallResult
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("uninstall did not return JSON: %v: %s", err, stdout.String())
	}
	if !result.OK {
		t.Fatalf("uninstall failed: %+v", result)
	}
	for _, path := range []string{binary, skill} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed, err = %v", path, err)
		}
	}
}
