package safety

import (
	"path/filepath"
	"strings"
)

type Analysis struct {
	Dangerous bool
	Action    string
	Reason    string
}

func Analyze(argv []string, stdin string) Analysis {
	if len(argv) > 0 {
		return analyzeCommand(argv)
	}
	for _, line := range strings.Split(stdin, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, segment := range splitCommandSegments(line) {
			if res := analyzeCommand(shellFields(segment)); res.Dangerous {
				return res
			}
		}
	}
	return Analysis{}
}

func splitCommandSegments(line string) []string {
	var segments []string
	start := 0
	inSingle := false
	inDouble := false
	escaped := false

	flush := func(end int) {
		segment := strings.TrimSpace(line[start:end])
		if segment != "" {
			segments = append(segments, segment)
		}
	}

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if inSingle || inDouble {
			continue
		}
		switch ch {
		case ';', '|', '&':
			flush(i)
			if i+1 < len(line) && line[i+1] == ch {
				i++
			}
			start = i + 1
		}
	}
	flush(len(line))
	return segments
}

func shellFields(line string) []string {
	var fields []string
	var b strings.Builder
	inSingle := false
	inDouble := false
	escaped := false
	haveField := false

	flush := func() {
		if haveField {
			fields = append(fields, b.String())
			b.Reset()
			haveField = false
		}
	}

	for i := 0; i < len(line); i++ {
		ch := line[i]
		if escaped {
			b.WriteByte(ch)
			haveField = true
			escaped = false
			continue
		}
		if ch == '\\' && !inSingle {
			escaped = true
			haveField = true
			continue
		}
		if ch == '\'' && !inDouble {
			inSingle = !inSingle
			haveField = true
			continue
		}
		if ch == '"' && !inSingle {
			inDouble = !inDouble
			haveField = true
			continue
		}
		if (ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n') && !inSingle && !inDouble {
			flush()
			continue
		}
		b.WriteByte(ch)
		haveField = true
	}
	flush()
	return fields
}

func analyzeCommand(argv []string) Analysis {
	argv = normalizePrefix(argv)
	if len(argv) == 0 {
		return Analysis{}
	}
	cmd := filepath.Base(argv[0])
	if hasBroadGlob(argv[1:]) {
		return dangerous("broad glob", "command includes a broad glob pattern")
	}
	switch cmd {
	case "rm", "unlink", "rmdir":
		return dangerous("delete files", "file deletion can remove workspace data")
	case "chmod":
		return dangerous("change permissions", "chmod can make files inaccessible or executable")
	case "chown", "chgrp":
		return dangerous("change ownership", "ownership changes can break local access")
	case "mv":
		if overwriteProne(argv[1:]) {
			return dangerous("move files", "mv can overwrite an existing destination")
		}
	case "cp":
		if overwriteProne(argv[1:]) {
			return dangerous("copy files", "cp can overwrite an existing destination")
		}
	case "git":
		return analyzeGit(argv[1:])
	case "sh", "bash", "dash", "zsh", "ksh":
		return analyzeShell(argv[1:])
	case "npm", "pnpm":
		return analyzePackage(cmd, argv[1:], []string{"uninstall", "remove", "rm"})
	case "yarn":
		return analyzePackage(cmd, argv[1:], []string{"remove"})
	case "brew":
		return analyzePackage(cmd, argv[1:], []string{"uninstall", "remove"})
	case "apt", "apt-get", "dnf", "yum":
		return analyzePackage(cmd, argv[1:], []string{"remove", "purge", "autoremove"})
	case "pacman":
		return analyzePackage(cmd, argv[1:], []string{"-R", "-Rs", "-Rns"})
	case "pip", "pip3", "gem", "cargo":
		return analyzePackage(cmd, argv[1:], []string{"uninstall"})
	}
	return Analysis{}
}

func normalizePrefix(argv []string) []string {
	for len(argv) > 0 {
		switch filepath.Base(argv[0]) {
		case "sudo", "doas":
			argv = stripWrapperOptions(argv[1:])
		case "command":
			argv = stripCommandOptions(argv[1:])
		default:
			return argv
		}
	}
	return argv
}

func stripWrapperOptions(args []string) []string {
	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			return args[1:]
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return args
		}
		args = args[1:]
		if wrapperOptionNeedsNextValue(arg) && len(args) > 0 {
			args = args[1:]
		}
	}
	return args
}

func wrapperOptionNeedsNextValue(arg string) bool {
	if strings.HasPrefix(arg, "--") {
		name := strings.TrimPrefix(arg, "--")
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			return false
		}
		switch name {
		case "chdir", "close-from", "group", "host", "login-class", "prompt", "role", "type", "user":
			return true
		default:
			return false
		}
	}
	valueOptions := "CDghpTtUu"
	for i := 1; i < len(arg); i++ {
		if strings.ContainsRune(valueOptions, rune(arg[i])) {
			return i == len(arg)-1
		}
	}
	return false
}

func stripCommandOptions(args []string) []string {
	for len(args) > 0 {
		switch args[0] {
		case "--":
			return args[1:]
		case "-p", "-v", "-V":
			args = args[1:]
		default:
			return args
		}
	}
	return args
}

func analyzeGit(args []string) Analysis {
	args = stripGitGlobalOptions(args)
	if len(args) == 0 {
		return Analysis{}
	}
	switch args[0] {
	case "reset":
		for _, arg := range args[1:] {
			if arg == "--hard" {
				return dangerous("reset git worktree", "git reset --hard discards local changes")
			}
		}
	case "clean":
		joined := strings.Join(args[1:], " ")
		if strings.Contains(joined, "-fdx") || strings.Contains(joined, "-xdf") ||
			(hasFlag(args[1:], "-f") && hasFlag(args[1:], "-d") && hasFlag(args[1:], "-x")) {
			return dangerous("clean git worktree", "git clean -fdx deletes untracked files")
		}
	}
	return Analysis{}
}

func stripGitGlobalOptions(args []string) []string {
	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			return args[1:]
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			return args
		}
		args = args[1:]
		if gitGlobalOptionNeedsNextValue(arg) && len(args) > 0 {
			args = args[1:]
		}
	}
	return args
}

func gitGlobalOptionNeedsNextValue(arg string) bool {
	if strings.HasPrefix(arg, "--") {
		name := strings.TrimPrefix(arg, "--")
		if idx := strings.IndexByte(name, '='); idx >= 0 {
			return false
		}
		switch name {
		case "exec-path", "git-dir", "namespace", "super-prefix", "work-tree":
			return true
		default:
			return false
		}
	}
	if arg == "-C" || arg == "-c" {
		return true
	}
	return false
}

func analyzeShell(args []string) Analysis {
	for len(args) > 0 {
		arg := args[0]
		if arg == "--" {
			args = args[1:]
			continue
		}
		if arg == "-c" {
			if len(args) < 2 {
				return Analysis{}
			}
			return Analyze(nil, args[1])
		}
		if strings.HasPrefix(arg, "-") && strings.Contains(arg[1:], "c") {
			if len(args) < 2 {
				return Analysis{}
			}
			return Analyze(nil, args[1])
		}
		if strings.HasPrefix(arg, "-") {
			args = args[1:]
			continue
		}
		return Analysis{}
	}
	return Analysis{}
}

func analyzePackage(cmd string, args []string, destructive []string) Analysis {
	if len(args) == 0 {
		return Analysis{}
	}
	for _, verb := range destructive {
		if args[0] == verb {
			return dangerous("remove packages", cmd+" "+verb+" can remove installed dependencies")
		}
	}
	return Analysis{}
}

func hasBroadGlob(args []string) bool {
	for _, arg := range args {
		if strings.ContainsAny(arg, "*?[") {
			return true
		}
	}
	return false
}

func overwriteProne(args []string) bool {
	operands := 0
	for _, arg := range args {
		if arg == "-n" || arg == "-i" {
			return false
		}
		if strings.HasPrefix(arg, "-") {
			continue
		}
		operands++
	}
	return operands >= 2
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag || strings.Contains(arg, strings.TrimPrefix(flag, "-")) && strings.HasPrefix(arg, "-") {
			return true
		}
	}
	return false
}

func dangerous(action, reason string) Analysis {
	return Analysis{Dangerous: true, Action: action, Reason: reason}
}
