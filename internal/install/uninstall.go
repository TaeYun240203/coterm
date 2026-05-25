package install

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Options struct {
	Home         string
	Purge        bool
	BinaryPath   string
	SkillDir     string
	PurgeTargets []string
}

type Result struct {
	Removed []string
	Skipped []string
}

func Uninstall(options Options) (Result, error) {
	home := options.Home
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return Result{}, err
		}
	}
	home, err := filepath.Abs(home)
	if err != nil {
		return Result{}, err
	}
	expectedBinary := filepath.Join(home, ".local", "bin", "coterm")
	expectedSkill := filepath.Join(home, ".codex", "skills", "coterm-shared-terminal")
	binaryPath := defaultPath(options.BinaryPath, expectedBinary)
	skillDir := defaultPath(options.SkillDir, expectedSkill)

	targets := []string{binaryPath, skillDir}
	if options.Purge {
		targets = append(targets, purgeTargets(home, options.PurgeTargets)...)
	}
	if err := guardTargets(home, targets, expectedBinary, expectedSkill); err != nil {
		return Result{}, err
	}

	var result Result
	for _, target := range targets {
		if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
			result.Skipped = append(result.Skipped, target)
			continue
		} else if err != nil {
			return result, err
		}
		if err := os.RemoveAll(target); err != nil {
			return result, err
		}
		if _, err := os.Lstat(target); errors.Is(err, os.ErrNotExist) {
			result.Removed = append(result.Removed, target)
		} else if err == nil {
			result.Skipped = append(result.Skipped, target)
		} else {
			return result, err
		}
	}
	return result, nil
}

func purgeTargets(home string, override []string) []string {
	if len(override) > 0 {
		return append([]string(nil), override...)
	}
	return []string{
		filepath.Join(home, ".cache", "coterm"),
		filepath.Join(home, ".local", "state", "coterm"),
		filepath.Join(home, ".config", "coterm"),
	}
}

func guardTargets(home string, targets []string, expectedBinary, expectedSkill string) error {
	expected := map[string]bool{
		clean(expectedBinary):                                   true,
		clean(expectedSkill):                                    true,
		clean(filepath.Join(home, ".cache", "coterm")):          true,
		clean(filepath.Join(home, ".local", "state", "coterm")): true,
		clean(filepath.Join(home, ".config", "coterm")):         true,
	}
	for _, target := range targets {
		cleaned := clean(target)
		if !filepath.IsAbs(target) {
			return errors.New("uninstall target must be absolute")
		}
		if !expected[cleaned] {
			return errors.New("refusing to remove unexpected path: " + target)
		}
		if strings.Contains(cleaned, string(filepath.Separator)+".coterm") {
			return errors.New("refusing to remove workspace .coterm path: " + target)
		}
	}
	return nil
}

func defaultPath(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

func clean(path string) string {
	cleaned, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return filepath.Clean(path)
	}
	return cleaned
}
