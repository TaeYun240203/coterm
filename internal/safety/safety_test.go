package safety

import "testing"

func TestDetectDangerousCommands(t *testing.T) {
	cases := [][]string{
		{"rm", "-rf", "dist"},
		{"chmod", "-R", "777", "."},
		{"chown", "-R", "me", "."},
		{"git", "reset", "--hard"},
		{"git", "clean", "-fdx"},
		{"npm", "uninstall", "react"},
		{"brew", "uninstall", "node"},
		{"mv", "build", "dist"},
		{"cp", "a.txt", "b.txt"},
		{"rm", "*.log"},
	}
	for _, args := range cases {
		if res := Analyze(args, ""); !res.Dangerous {
			t.Fatalf("Analyze(%v) not dangerous", args)
		}
	}
}

func TestAllowsOrdinaryCommands(t *testing.T) {
	for _, args := range [][]string{{"npm", "test"}, {"git", "status"}, {"go", "test", "./..."}} {
		if res := Analyze(args, ""); res.Dangerous {
			t.Fatalf("Analyze(%v) dangerous: %s", args, res.Reason)
		}
	}
}

func TestAnalyzeStdinScansTrimmedNonCommentLines(t *testing.T) {
	script := `
		# rm -rf ignored
		printf 'hello'
		  git reset --hard
	`
	res := Analyze(nil, script)
	if !res.Dangerous {
		t.Fatalf("Analyze(stdin) not dangerous")
	}
	if res.Action == "" || res.Reason == "" {
		t.Fatalf("Analyze(stdin) missing action/reason: %+v", res)
	}
}
