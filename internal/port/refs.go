package port

import "strings"

// TagNames parses the output of `git tag -l` into tag names. A name starting with "-" is dropped:
// git itself refuses to create one, but `git update-ref` or a fetch from a hostile remote can, and
// the name would then be read as an option by every git command that receives it as an argument.
func TagNames(stdout string) []string {
	var tags []string
	for line := range strings.SplitSeq(stdout, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "-") {
			tags = append(tags, line)
		}
	}
	return tags
}
