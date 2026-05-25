package safety

import "testing"

func TestDetectDangerousCommands(t *testing.T) {
	cases := [][]string{
		{"rm", "-rf", "dist"},
		{"chmod", "-R", "777", "."},
		{"chown", "-R", "me", "."},
		{"git", "reset", "--hard"},
		{"git", "clean", "-fdx"},
		{"git", "clean", "-fd"},
		{"git", "clean", "-f", "-d"},
		{"sudo", "-E", "rm", "-rf", "dist"},
		{"sudo", "--preserve-env", "rm", "-rf", "dist"},
		{"sudo", "-Eu", "root", "rm", "-rf", "dist"},
		{"git", "-C", "repo", "reset", "--hard"},
		{"git", "-c", "x=y", "clean", "-fdx"},
		{"git", "-c", "alias.z=!rm -rf dist", "z"},
		{"git", "-calias.z=!rm -rf dist", "z"},
		{"sh", "-c", "rm -rf dist"},
		{"bash", "-lc", "git reset --hard"},
		{"bash", "-o", "pipefail", "-c", "rm -rf dist"},
		{"env", "rm", "-rf", "dist"},
		{"env", "-i", "bash", "-lc", "git reset --hard"},
		{"env", "NODE_ENV=test", "rm", "-rf", "dist"},
		{"env", "-u", "PATH", "git", "clean", "-fd"},
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
	for _, args := range [][]string{
		{"npm", "test"},
		{"git", "status"},
		{"git", "clean", "-nd"},
		{"command", "-v", "rm"},
		{"command", "-V", "rm"},
		{"env", "NODE_ENV=test"},
		{"go", "test", "./..."},
	} {
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

func TestAnalyzeStdinScansCommandSegments(t *testing.T) {
	cases := []string{
		"echo ok; rm -rf dist",
		"true && git reset --hard",
		"false || git clean -fdx",
		"cat files.txt | rm -rf dist",
		"sh -c 'rm -rf dist'",
		"sh -c 'echo $(rm -rf dist)'",
		"echo `git reset --hard`",
		`bash -lc "git reset --hard"`,
		`bash -o pipefail -c "rm -rf dist"`,
	}
	for _, script := range cases {
		if res := Analyze(nil, script); !res.Dangerous {
			t.Fatalf("Analyze(%q) not dangerous", script)
		}
	}
}

func TestAnalyzeStdinIgnoresSeparatorsInsideSimpleQuotes(t *testing.T) {
	script := `printf '%s\n' 'echo ok; rm -rf dist'`
	if res := Analyze(nil, script); res.Dangerous {
		t.Fatalf("Analyze(%q) dangerous: %s", script, res.Reason)
	}
}

func TestAnalyzeStdinIgnoresSubstitutionsInsideSingleQuotes(t *testing.T) {
	script := `printf '%s\n' '$(rm -rf dist)'`
	if res := Analyze(nil, script); res.Dangerous {
		t.Fatalf("Analyze(%q) dangerous: %s", script, res.Reason)
	}
}
