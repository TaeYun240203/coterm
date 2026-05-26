package install

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUninstallRemovesOnlyExpectedInstallPaths(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	binary := filepath.Join(home, ".local", "bin", "coterm")
	skill := filepath.Join(home, ".codex", "skills", "coterm-shared-terminal")
	otherBinary := filepath.Join(home, ".local", "bin", "other")
	workspaceState := filepath.Join(root, "workspace", ".coterm")
	for _, dir := range []string{filepath.Dir(binary), skill, filepath.Dir(otherBinary), workspaceState} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{binary, filepath.Join(skill, "SKILL.md"), otherBinary, filepath.Join(workspaceState, "session.toml")} {
		if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	result, err := Uninstall(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Removed) != 2 {
		t.Fatalf("removed = %+v, want binary and skill", result.Removed)
	}
	for _, path := range []string{binary, skill} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed, err = %v", path, err)
		}
	}
	for _, path := range []string{otherBinary, filepath.Join(workspaceState, "session.toml")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected %s preserved: %v", path, err)
		}
	}
}

func TestUninstallPurgeRemovesOnlyGlobalCotermData(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(root, "home")
	for _, dir := range []string{
		filepath.Join(home, ".cache", "coterm"),
		filepath.Join(home, ".local", "state", "coterm"),
		filepath.Join(root, "workspace", ".coterm"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	workspaceFile := filepath.Join(root, "workspace", ".coterm", "commands.jsonl")
	if err := os.WriteFile(workspaceFile, []byte("keep"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Uninstall(Options{Home: home, Purge: true}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(home, ".cache", "coterm"),
		filepath.Join(home, ".local", "state", "coterm"),
	} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expected %s removed, err = %v", path, err)
		}
	}
	if _, err := os.Stat(workspaceFile); err != nil {
		t.Fatalf("purge removed workspace state: %v", err)
	}
}

func TestUninstallRejectsUnexpectedPathEscapes(t *testing.T) {
	_, err := Uninstall(Options{
		Home:         t.TempDir(),
		BinaryPath:   "/tmp/coterm",
		SkillDir:     "relative-skill",
		PurgeTargets: []string{"/tmp/coterm-cache"},
		Purge:        true,
	})
	if err == nil {
		t.Fatal("expected guarded path rejection")
	}
}

func TestUninstallRemovesBrokenSymlinkAtExpectedPath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	binary := filepath.Join(home, ".local", "bin", "coterm")
	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(home, "missing-target"), binary); err != nil {
		t.Fatal(err)
	}

	result, err := Uninstall(Options{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(binary); !os.IsNotExist(err) {
		t.Fatalf("expected broken symlink removed, err = %v", err)
	}
	if len(result.Removed) == 0 {
		t.Fatalf("removed = %+v, want broken symlink listed", result.Removed)
	}
}
