package config

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
)

// IncrementSprint reads the config file at path, increments versioning.sprint by
// one, writes the file back preserving all other content, and returns the new value.
// If sprint is absent from the file (default 0), it is inserted and set to 1.
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

	// Fast path: replace existing `sprint: N` in-place to preserve formatting.
	re := regexp.MustCompile(`(?m)^(\s+sprint:\s*)\d+`)
	if re.Match(data) {
		updated := re.ReplaceAllFunc(data, func(match []byte) []byte {
			sub := re.FindSubmatch(match)
			return append(sub[1], strconv.Itoa(newSprint)...)
		})
		return newSprint, os.WriteFile(path, updated, 0o644)
	}

	// Slow path: sprint: is missing (was 0 / omitempty). Insert it into the
	// versioning block right after the `versioning:` key line.
	versioningRe := regexp.MustCompile(`(?m)^versioning:\s*$`)
	loc := versioningRe.FindIndex(data)
	if loc == nil {
		return 0, fmt.Errorf("versioning: section not found in %s", path)
	}
	// Find the end of the versioning: line and insert after it.
	insertAt := loc[1]
	indent := detectIndent(data[insertAt:])
	insertion := fmt.Sprintf("\n%ssprint: %d", indent, newSprint)
	updated := make([]byte, 0, len(data)+len(insertion))
	updated = append(updated, data[:insertAt]...)
	updated = append(updated, insertion...)
	updated = append(updated, data[insertAt:]...)
	return newSprint, os.WriteFile(path, updated, 0o644)
}

// detectIndent returns the leading whitespace of the first non-empty line in s,
// used to match the indentation style already used in the versioning block.
func detectIndent(s []byte) string {
	for _, line := range regexp.MustCompile(`\n`).Split(string(s), -1) {
		if len(line) == 0 {
			continue
		}
		indent := ""
		for _, ch := range line {
			if ch == ' ' || ch == '\t' {
				indent += string(ch)
			} else {
				break
			}
		}
		if indent != "" {
			return indent
		}
	}
	return "  " // default 2-space
}
