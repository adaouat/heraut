package heraut_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	fencedBlock  = regexp.MustCompile("(?s)```.*?```")
	inlineCode   = regexp.MustCompile("`[^`]*`")
	markdownLink = regexp.MustCompile(`(?:^|[^\\])\[[^\]\n]*\]\(([^)\s]+)\)`)
	heading      = regexp.MustCompile(`(?m)^#{1,6}\s+(.+?)\s*$`)
)

// userPages are the pages written for people who use heraut. They link to each other and to the
// reference, never to the specs or the ADRs, which are written for contributors.
func userPages(t *testing.T) []string {
	t.Helper()
	pages := []string{"README.md", "docs/README.md", "docs/install.md", "docs/getting-started.md"}
	for _, dir := range []string{"docs/guides", "docs/concepts", "docs/reference", "docs/reference/commands"} {
		found, err := filepath.Glob(filepath.Join(dir, "*.md"))
		require.NoError(t, err)
		pages = append(pages, found...)
	}
	return pages
}

// historical holds snapshots kept as written: archived roadmaps and old design notes, whose links
// point at files that moved or were removed since.
var historical = []string{"docs/tasks/archive", "docs/superpowers"}

// allPages is every living Markdown file of the repository's documentation.
func allPages(t *testing.T) []string {
	t.Helper()
	pages := []string{"README.md", "CONTRIBUTING.md", "CLAUDE.md"}
	err := filepath.WalkDir("docs", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		for _, h := range historical {
			if d.IsDir() && path == h {
				return filepath.SkipDir
			}
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			pages = append(pages, path)
		}
		return nil
	})
	require.NoError(t, err)
	return pages
}

// slug turns a heading into the anchor GitHub gives it: lower case, punctuation dropped, spaces as
// dashes, and -1, -2 … for repeats.
func slug(text string) string {
	text = strings.ToLower(strings.ReplaceAll(text, "`", ""))
	var b strings.Builder
	for _, r := range text {
		switch {
		case r == ' ':
			b.WriteRune('-')
		case r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r > 127 && isLetter(r):
			b.WriteRune(r)
		}
	}
	return b.String()
}

func isLetter(r rune) bool {
	return strings.ContainsRune("àâäéèêëîïôöùûüçñßæøåœ", r)
}

func anchorsOf(t *testing.T, path string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := fencedBlock.ReplaceAllString(string(raw), "")
	anchors := map[string]bool{}
	seen := map[string]int{}
	for _, m := range heading.FindAllStringSubmatch(text, -1) {
		s := slug(m[1])
		if n := seen[s]; n > 0 {
			anchors[s+"-"+itoa(n)] = true
		} else {
			anchors[s] = true
		}
		seen[s]++
	}
	return anchors
}

func itoa(n int) string { return strconv.Itoa(n) }

// links returns the relative links of a page, outside code.
func links(t *testing.T, path string) []string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	text := inlineCode.ReplaceAllString(fencedBlock.ReplaceAllString(string(raw), ""), "")
	var out []string
	for _, m := range markdownLink.FindAllStringSubmatch(text, -1) {
		target := m[1]
		if strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
			continue
		}
		out = append(out, target)
	}
	return out
}

// Every relative link in the documentation points at a file that exists, and at a heading that
// exists when it names one.
func TestDocumentationLinks_Resolve(t *testing.T) {
	for _, page := range allPages(t) {
		for _, target := range links(t, page) {
			file, anchor, _ := strings.Cut(target, "#")
			dest := page
			if file != "" {
				dest = filepath.Join(filepath.Dir(page), file)
			}
			info, err := os.Stat(dest)
			if !assert.NoError(t, err, "%s links to %s, which does not exist", page, target) {
				continue
			}
			if anchor != "" && !info.IsDir() && strings.HasSuffix(dest, ".md") {
				assert.True(t, anchorsOf(t, dest)[anchor], "%s links to %s, but %s has no heading #%s", page, target, dest, anchor)
			}
		}
	}
}

// User pages never send their readers into the specs or the ADRs.
func TestDocumentationLinks_UserPagesStayInTheUserDocs(t *testing.T) {
	for _, page := range userPages(t) {
		for _, target := range links(t, page) {
			assert.NotRegexp(t, `(^|/)(specs|adr)/`, target, "%s is a user page but links to %s", page, target)
		}
	}
}
