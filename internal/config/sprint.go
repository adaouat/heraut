package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
)

var (
	versioningLineRe = regexp.MustCompile(`^versioning:[ \t]*(#.*)?$`)
	sprintValueRe    = regexp.MustCompile(`^(sprint:[ \t]*)\d+`)
)

// IncrementSprint reads the config file at path, increments versioning.sprint by
// one, writes the file back preserving all other content, and returns the new value.
// If sprint is absent from the file (default 0), it is inserted and set to 1.
//
// The edit is confined to the top-level versioning block: a "sprint:" line anywhere else (for
// example inside a hook script) is left alone. The block must be written in block style.
func IncrementSprint(path string) (int, error) {
	cfg, err := Load(path)
	if err != nil {
		return 0, err
	}

	newSprint := cfg.Versioning.Sprint + 1

	data, err := os.ReadFile(path)
	if err != nil {
		return 0, fmt.Errorf("reading config file: %w", err)
	}

	lines := strings.SplitAfter(string(data), "\n")
	start := -1
	for i, line := range lines {
		if versioningLineRe.MatchString(strings.TrimRight(line, "\r\n")) {
			start = i
			break
		}
	}
	if start < 0 {
		return 0, fmt.Errorf("versioning: block not found in %s: write it as a block (versioning:, then indented keys), not inline", path)
	}

	indent := ""
	for i := start + 1; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], "\r\n")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			break // next top-level key: the block ends here
		}
		if indent == "" {
			indent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		}
		lead := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		if lead == indent && sprintValueRe.MatchString(trimmed) {
			lines[i] = lead + sprintValueRe.ReplaceAllString(strings.TrimLeft(lines[i], " \t"), "${1}"+strconv.Itoa(newSprint))
			return newSprint, os.WriteFile(path, []byte(strings.Join(lines, "")), 0o644)
		}
	}

	if indent == "" {
		indent = "  "
	}
	insert := indent + "sprint: " + strconv.Itoa(newSprint) + "\n"
	if !strings.HasSuffix(lines[start], "\n") {
		lines[start] += "\n"
	}
	lines[start] += insert
	return newSprint, os.WriteFile(path, []byte(strings.Join(lines, "")), 0o644)
}
