package workspace

import (
	"errors"
	"os"
	"path/filepath"
)

func FindRoot(start string) (string, error) {
	if start == "" {
		start = "."
	}
	resolved, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		resolved = filepath.Dir(resolved)
	}

	fallback := resolved
	for {
		if _, err := os.Stat(filepath.Join(resolved, ".git")); err == nil {
			return resolved, nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}

		parent := filepath.Dir(resolved)
		if parent == resolved {
			return fallback, nil
		}
		resolved = parent
	}
}
