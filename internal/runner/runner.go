package runner

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func BuildArgvScript(commandID, workspace string, argv []string) string {
	var b strings.Builder
	writeHeader(&b, commandID, workspace)
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		quoted = append(quoted, ShellQuote(arg))
	}
	fmt.Fprintf(&b, "%s\n", strings.Join(quoted, " "))
	writeFooter(&b, commandID)
	return b.String()
}

func BuildStdinScript(commandID, workspace, payload string) string {
	var b strings.Builder
	writeHeader(&b, commandID, workspace)
	b.WriteString(payload)
	if !strings.HasSuffix(payload, "\n") {
		b.WriteByte('\n')
	}
	writeFooter(&b, commandID)
	return b.String()
}

func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func ParseExitCode(output, commandID string) (int, bool) {
	re := regexp.MustCompile(`__COTERM_EXIT_` + regexp.QuoteMeta(commandID) + `__:(-?[0-9]+)`)
	match := re.FindStringSubmatch(output)
	if len(match) != 2 {
		return 0, false
	}
	code, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, false
	}
	return code, true
}

func writeHeader(b *strings.Builder, commandID, workspace string) {
	fmt.Fprintf(b, "#!/bin/sh\n")
	fmt.Fprintf(b, "printf '\\n__COTERM_START_%s__\\n'\n", commandID)
	fmt.Fprintf(b, "cd %s || exit 127\n", ShellQuote(workspace))
}

func writeFooter(b *strings.Builder, commandID string) {
	fmt.Fprintf(b, "code=$?\n")
	fmt.Fprintf(b, "printf '\\n__COTERM_EXIT_%s__:%%s\\n' \"$code\"\n", commandID)
	fmt.Fprintf(b, "exit \"$code\"\n")
}
