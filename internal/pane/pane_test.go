package pane

import "testing"

func TestRecommendedNamesReturnsCopy(t *testing.T) {
	names := RecommendedNames()
	if len(names) == 0 {
		t.Fatal("RecommendedNames returned no names")
	}

	names[0] = "mutated"
	next := RecommendedNames()
	if next[0] == "mutated" {
		t.Fatal("RecommendedNames exposed mutable backing storage")
	}
}

func TestValidateAcceptsRecommendedNames(t *testing.T) {
	for _, name := range RecommendedNames() {
		t.Run(name, func(t *testing.T) {
			if err := ValidateName(name); err != nil {
				t.Fatalf("ValidateName(%q) = %v", name, err)
			}
		})
	}
}

func TestValidateAcceptsCustomNames(t *testing.T) {
	tests := []string{
		"a",
		"abcdefghijklmnopqrstuvwxyzabcdef",
		"build",
		"test_linux",
		"api-server",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateName(name); err != nil {
				t.Fatalf("ValidateName(%q) = %v", name, err)
			}
		})
	}
}

func TestValidateRejectsReservedAndInvalidNames(t *testing.T) {
	tests := []string{
		"",
		"Main1",
		"1main",
		"abcdefghijklmnopqrstuvwxyzabcdefg",
		"permission",
		"permission1",
		"permission-",
		"permission-build",
		"bad.name",
		"bad/name",
		"bad name",
		"bad:name",
	}
	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			if err := ValidateName(name); err == nil {
				t.Fatalf("ValidateName(%q) succeeded, want error", name)
			}
		})
	}
}

func TestValidateNameErrorQuotesName(t *testing.T) {
	err := ValidateName("Bad")
	if err == nil {
		t.Fatal("ValidateName succeeded, want error")
	}
	if got, want := err.Error(), `invalid pane name: "Bad"`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
