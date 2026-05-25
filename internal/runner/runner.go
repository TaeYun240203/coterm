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
	writeWorkspaceEnter(&b, workspace)
	fmt.Fprintf(&b, "if [ \"$coterm_code\" -eq 0 ]; then\n")
	fmt.Fprintf(&b, "(\n")
	quoted := make([]string, 0, len(argv))
	for _, arg := range argv {
		quoted = append(quoted, ShellQuote(arg))
	}
	fmt.Fprintf(&b, "%s\n", strings.Join(quoted, " "))
	fmt.Fprintf(&b, ")\n")
	fmt.Fprintf(&b, "coterm_code=$?\n")
	fmt.Fprintf(&b, "fi\n")
	writeFooter(&b, commandID)
	return b.String()
}

func BuildStdinScript(commandID, workspace, payload string) string {
	var b strings.Builder
	writeHeader(&b, commandID, workspace)
	writeWorkspaceEnter(&b, workspace)
	fmt.Fprintf(&b, "if [ \"$coterm_code\" -eq 0 ]; then\n")
	fmt.Fprintf(&b, "(\n")
	b.WriteString(payload)
	if !strings.HasSuffix(payload, "\n") {
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, ")\n")
	fmt.Fprintf(&b, "coterm_code=$?\n")
	fmt.Fprintf(&b, "fi\n")
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
}

func writeWorkspaceEnter(b *strings.Builder, workspace string) {
	fmt.Fprintf(b, "cd %s\n", ShellQuote(workspace))
	fmt.Fprintf(b, "coterm_code=$?\n")
}

func writeFooter(b *strings.Builder, commandID string) {
	fmt.Fprintf(b, "printf '\\n__COTERM_EXIT_%s__:%%s\\n' \"$coterm_code\"\n", commandID)
	fmt.Fprintf(b, "exit \"$coterm_code\"\n")
}
