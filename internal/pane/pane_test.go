package pane

import "testing"

func TestValidateRecommendedAndCustomNames(t *testing.T) {
	for _, name := range []string{"main1", "scratch2", "longrun1", "interactive2", "build", "test_linux", "api-server"} {
		if err := ValidateName(name); err != nil {
			t.Fatalf("ValidateName(%q) = %v", name, err)
		}
	}
}

func TestValidateRejectsReservedAndInvalidNames(t *testing.T) {
	for _, name := range []string{"", "Main1", "1main", "permission", "permission1", "permission-build", "bad.name", "bad/name"} {
		if err := ValidateName(name); err == nil {
			t.Fatalf("ValidateName(%q) succeeded, want error", name)
		}
	}
}
