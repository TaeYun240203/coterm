package state

import (
	"os"

	"github.com/pelletier/go-toml/v2"
)

func SaveTOML(path string, value any) error {
	data, err := toml.Marshal(value)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadTOML(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return toml.Unmarshal(data, value)
}
