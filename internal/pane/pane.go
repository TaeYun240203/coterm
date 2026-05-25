package pane

import (
	"fmt"
	"regexp"
	"strings"
)

var Recommended = []string{"main1", "main2", "scratch1", "scratch2", "longrun1", "longrun2", "interactive1", "interactive2"}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func ValidateName(name string) error {
	if name == "permission" || name == "permission1" || strings.HasPrefix(name, "permission-") {
		return fmt.Errorf("reserved pane name: %s", name)
	}
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid pane name: %s", name)
	}
	return nil
}
