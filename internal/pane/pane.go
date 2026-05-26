package pane

import (
	"fmt"
	"regexp"
	"strings"
)

var recommended = []string{"main1", "main2", "scratch1", "scratch2", "longrun1", "longrun2", "interactive1", "interactive2"}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func RecommendedNames() []string {
	return append([]string(nil), recommended...)
}

func ValidateName(name string) error {
	if IsReserved(name) {
		return fmt.Errorf("reserved pane name: %q", name)
	}
	if !nameRE.MatchString(name) {
		return fmt.Errorf("invalid pane name: %q", name)
	}
	return nil
}

func IsReserved(name string) bool {
	return name == "permission" || name == "permission1" || strings.HasPrefix(name, "permission-")
}
