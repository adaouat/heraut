package native

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── fixtures ───────────────────────────────────────────────────────────────

var (
	fixedDate1 = time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	fixedDate2 = time.Date(2024, 1, 14, 12, 0, 0, 0, time.UTC)
	fixedDate3 = time.Date(2024, 1, 10, 12, 0, 0, 0, time.UTC)

	// fixtureHeraut feeds the golden tests below a realistic .Heraut value so the default
	// footer (which now renders it) produces legible, deterministic golden files instead of
	// zero-value output.
	fixtureHeraut = tplHeraut{Version: "1.2.3", URL: herautProjectURL, GeneratedAt: fixedDate1}

	githubLC = &port.LinkContext{
		BaseURL:  "https://github.com",
		Owner:    "acme",
		Repo:     "widget",
		Platform: "github",
	}

	// Three deterministic raw commits used across golden tests.
	// c1: scoped feat, date fixedDate3 (oldest)
	rc1 = rawCommit{
		Hash:    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Author:  "Alice",
		Email:   "alice@example.com",
		Date:    fixedDate3,
		Subject: "feat(api): add new endpoint",
		Body:    "",
	}
	// c2: unscoped fix, date fixedDate2
	rc2 = rawCommit{
		Hash:    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Author:  "Bob",
		Email:   "bob@example.com",
		Date:    fixedDate2,
		Subject: "fix: prevent null pointer",
		Body:    "",
	}
	// c3: scoped feat with breaking flag, date fixedDate1 (newest)
	rc3 = rawCommit{
		Hash:    "cccccccccccccccccccccccccccccccccccccccc",
		Author:  "Carol",
		Email:   "carol@example.com",
		Date:    fixedDate1,
		Subject: "feat(ui)!: redesign landing page",
		Body:    "",
	}
	// c4: commit with body and footer (for release-notes body/footer test)
	rc4 = rawCommit{
		Hash:    "dddddddddddddddddddddddddddddddddddddddd",
		Author:  "Dave",
		Email:   "dave@example.com",
		Date:    fixedDate2,
		Subject: "feat(auth): add oauth2 support",
		Body:    "Implements the OAuth2 authorization_code flow.\n\nCo-Authored-By: Eve <eve@example.com>",
	}
)

// fixtureGroups builds a []group from the three base commits.
// Features: c1 (scoped api) + c3 (scoped ui, breaking) — scope-sorted: api before ui
// Bug Fixes: c2 (unscoped fix)
func fixtureGroups() []group {
	pc1 := parsedCommit{raw: rc1}
	pc1.parsed, _ = conventionalcommit.Parse(rc1.Subject)

	pc3 := parsedCommit{raw: rc3}
	pc3.parsed, _ = conventionalcommit.Parse(rc3.Subject)

	pc2 := parsedCommit{raw: rc2}
	pc2.parsed, _ = conventionalcommit.Parse(rc2.Subject)

	return []group{
		{name: "🚀 Features", order: 0, commits: []parsedCommit{pc1, pc3}},
		{name: "🐛 Bug Fixes", order: 1, commits: []parsedCommit{pc2}},
	}
}

// fixtureGroupsWithBodyFooter builds a group containing a commit with body + footer.
// Parse uses the full message (subject + body) so Body and Footers are populated.
func fixtureGroupsWithBodyFooter() []group {
	pc4 := parsedCommit{raw: rc4}
	pc4.parsed, _ = conventionalcommit.Parse(commitMessage(rc4))

	return []group{
		{name: "🚀 Features", order: 0, commits: []parsedCommit{pc4}},
	}
}

// readGolden reads a golden file relative to testdata/; calls t.Fatal if missing.
func readGolden(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	require.NoError(t, err, "golden file %q not found — run with UPDATE_GOLDEN=1 to create", name)
	return string(data)
}

// writeGolden writes content to testdata/<name> when UPDATE_GOLDEN=1.
func writeGolden(t *testing.T, name, content string) {
	t.Helper()
	if os.Getenv("UPDATE_GOLDEN") != "1" {
		return
	}
	path := filepath.Join("testdata", name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// ─── changelog golden test ───────────────────────────────────────────────────

func TestRenderChangelogSection_Golden(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := renderChangelogSection(
		"v1.2.3", "v1.2.2", releaseDate,
		fixtureGroups(), githubLC, nil, "", 3, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "changelog_github.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

func TestRenderChangelogSection_NoPrevious(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := renderChangelogSection(
		"v1.0.0", "", releaseDate,
		fixtureGroups(), githubLC, nil, "", 3, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "changelog_no_previous.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

func TestRenderChangelogSection_NoLinks(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := renderChangelogSection(
		"v1.2.3", "v1.2.2", releaseDate,
		fixtureGroups(), nil, nil, "", 3, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "changelog_no_links.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

func TestRenderChangelogSection_WithTickets(t *testing.T) {
	rcTicket := rawCommit{
		Hash:    "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Author:  "Eve",
		Email:   "eve@example.com",
		Date:    fixedDate1,
		Subject: "fix: resolve PROJ-42 and PROJ-99",
		Body:    "",
	}
	pcTicket := parsedCommit{raw: rcTicket}
	pcTicket.parsed, _ = conventionalcommit.Parse(rcTicket.Subject)

	groups := []group{{name: "🐛 Bug Fixes", order: 1, commits: []parsedCommit{pcTicket}}}
	tickets := []config.Ticket{
		{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
	}

	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := renderChangelogSection(
		"v2.0.0", "v1.9.9", releaseDate,
		groups, githubLC, tickets, "", 3, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "changelog_with_tickets.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

// TestRenderChangelogSection_TicketBlockOverride covers T240: rendering.templates.ticket must
// override just the per-ticket link fragment, without requiring the caller to restate the whole
// commit block — the "ticket" block is called from "commit" via {{ template "ticket" . }}, so
// overriding it alone changes ticket rendering and nothing else about the commit line.
func TestRenderChangelogSection_TicketBlockOverride(t *testing.T) {
	rcTicket := rawCommit{
		Hash:    "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		Author:  "Eve",
		Email:   "eve@example.com",
		Date:    fixedDate1,
		Subject: "fix: resolve PROJ-42",
		Body:    "",
	}
	pcTicket := parsedCommit{raw: rcTicket}
	pcTicket.parsed, _ = conventionalcommit.Parse(rcTicket.Subject)

	groups := []group{{name: "🐛 Bug Fixes", order: 1, commits: []parsedCommit{pcTicket}}}
	tickets := []config.Ticket{
		{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
	}
	snippets := map[string]string{"ticket": "🎫[{{ .Text }}]({{ .Href }})"}

	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := renderChangelogSection(
		"v2.0.0", "v1.9.9", releaseDate,
		groups, githubLC, tickets, "", 3, nil, tplHeraut{}, snippets, "",
	)
	require.NoError(t, err)

	assert.Contains(t, got, "🎫[PROJ-42](https://jira.example.com/browse/PROJ-42)")
	assert.NotContains(t, got, "([PROJ-42]", "the default ( ) wrapper must be gone once overridden")
	assert.Contains(t, got, "Resolve PROJ-42", "the rest of the commit line must be untouched")
}

func TestRenderChangelogSection_HeadingPattern(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	// Simulates a per-env tag like "prod/v1.2.3" where the pattern strips "prod/".
	// The headingVersionPattern applied to the rendered heading replaces the full
	// version bracket contents matching group 1 with [$1].
	got, err := renderChangelogSection(
		"prod/v1.2.3", "prod/v1.2.2", releaseDate,
		fixtureGroups(), githubLC, nil,
		`\[prod/(v[^\]]+)\]`, 3, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "changelog_heading_pattern.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

func TestRenderChangelogSection_TypesHeadingLevel(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	got, err := renderChangelogSection(
		"v1.2.3", "v1.2.2", releaseDate,
		fixtureGroups(), githubLC, nil, "", 2, nil, tplHeraut{}, nil, "",
	)
	require.NoError(t, err)
	assert.Contains(t, got, "\n## 🚀 Features", "types_heading_level 2 → ## group headings")
	assert.NotContains(t, got, "### 🚀 Features")
}

// ─── release-notes golden test ───────────────────────────────────────────────

func TestRenderReleaseNotes_Golden(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	prevDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	got, err := renderReleaseNotes(
		"v1.2.3", "v1.2.2", releaseDate,
		fixtureGroups(), githubLC, nil, prevDate, 3, nil, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "release_notes_github.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

func TestRenderReleaseNotes_NoPrevDate(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	got, err := renderReleaseNotes(
		"v1.0.0", "", releaseDate,
		fixtureGroups(), githubLC, nil, time.Time{}, 3, nil, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "release_notes_no_prev_date.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

func TestRenderReleaseNotes_WithBodyAndFooter(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)

	got, err := renderReleaseNotes(
		"v1.1.0", "v1.0.0", releaseDate,
		fixtureGroupsWithBodyFooter(), githubLC, nil, time.Time{}, 3, nil, nil, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "release_notes_body_footer.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}

// ─── link composition table tests ────────────────────────────────────────────

func TestBuildCommitURL(t *testing.T) {
	tests := []struct {
		name string
		lc   *port.LinkContext
		want string
	}{
		{
			name: "nil → empty",
			lc:   nil,
			want: "",
		},
		{
			name: "github",
			lc:   &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
			want: "https://github.com/acme/widget/commit/",
		},
		{
			name: "github trailing slash stripped",
			lc:   &port.LinkContext{BaseURL: "https://github.com/", Owner: "acme", Repo: "widget", Platform: "github"},
			want: "https://github.com/acme/widget/commit/",
		},
		{
			name: "gitlab",
			lc:   &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"},
			want: "https://gitlab.com/group/sub/proj/-/commit/",
		},
		{
			name: "azure_devops",
			lc:   &port.LinkContext{BaseURL: "https://dev.azure.com", Owner: "org/proj", Repo: "repo", Platform: "azure_devops"},
			want: "https://dev.azure.com/org/proj/_git/repo/commit/",
		},
		{
			name: "azure_devops with spaces (path-escaped)",
			lc:   &port.LinkContext{BaseURL: "https://dev.azure.com", Owner: "my org/my proj", Repo: "my repo", Platform: "azure_devops"},
			want: "https://dev.azure.com/my%20org/my%20proj/_git/my%20repo/commit/",
		},
		{
			name: "ambient github (Owner/Repo empty)",
			lc:   &port.LinkContext{BaseURL: "https://github.com/acme/widget", Platform: "github"},
			want: "https://github.com/acme/widget/commit/",
		},
		{
			name: "ambient gitlab (Owner/Repo empty)",
			lc:   &port.LinkContext{BaseURL: "https://gitlab.com/grp/proj", Platform: "gitlab"},
			want: "https://gitlab.com/grp/proj/-/commit/",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildCommitURL(tc.lc))
		})
	}
}

func TestBuildCompareURL(t *testing.T) {
	tests := []struct {
		name    string
		lc      *port.LinkContext
		prev    string
		version string
		want    string
	}{
		{
			name:    "nil lc → empty",
			lc:      nil,
			prev:    "v1.0.0",
			version: "v1.1.0",
			want:    "",
		},
		{
			name:    "empty prev → empty",
			lc:      &port.LinkContext{BaseURL: "https://github.com", Owner: "a", Repo: "b", Platform: "github"},
			prev:    "",
			version: "v1.1.0",
			want:    "",
		},
		{
			name:    "github",
			lc:      &port.LinkContext{BaseURL: "https://github.com", Owner: "acme", Repo: "widget", Platform: "github"},
			prev:    "v1.0.0",
			version: "v1.1.0",
			want:    "https://github.com/acme/widget/compare/v1.0.0..v1.1.0",
		},
		{
			name:    "gitlab",
			lc:      &port.LinkContext{BaseURL: "https://gitlab.com", Owner: "group/sub", Repo: "proj", Platform: "gitlab"},
			prev:    "v1.0.0",
			version: "v1.1.0",
			want:    "https://gitlab.com/group/sub/proj/-/compare/v1.0.0..v1.1.0",
		},
		{
			name:    "azure_devops two-part compare URL",
			lc:      &port.LinkContext{BaseURL: "https://dev.azure.com", Owner: "org/proj", Repo: "repo", Platform: "azure_devops"},
			prev:    "v1.0.0",
			version: "v1.1.0",
			want:    "https://dev.azure.com/org/proj/_git/repo/branchCompare?baseVersion=GTv1.0.0&targetVersion=GTv1.1.0",
		},
		{
			name:    "ambient (no owner/repo)",
			lc:      &port.LinkContext{BaseURL: "https://github.com/acme/widget", Platform: "github"},
			prev:    "v1.0.0",
			version: "v1.1.0",
			want:    "https://github.com/acme/widget/compare/v1.0.0..v1.1.0",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, buildCompareURL(tc.lc, tc.prev, tc.version))
		})
	}
}

// ─── ticket link table tests ─────────────────────────────────────────────────

func TestResolveTickets(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		tickets []config.Ticket
		want    []ticketLink
	}{
		{
			name:    "no tickets configured",
			text:    "fix PROJ-42",
			tickets: nil,
			want:    nil,
		},
		{
			name:    "no pattern match",
			text:    "fix typo",
			tickets: []config.Ticket{{Pattern: `PROJ-\d+`, URL: "https://jira.example.com/browse/{ticket}"}},
			want:    nil,
		},
		{
			name: "pattern with no capture group uses full match as URL value",
			text: "fix PROJ-42",
			tickets: []config.Ticket{
				{Pattern: `PROJ-\d+`, URL: "https://jira.example.com/browse/{ticket}"},
			},
			want: []ticketLink{
				{Text: "PROJ-42", Href: "https://jira.example.com/browse/PROJ-42"},
			},
		},
		{
			name: "pattern with capture group uses group 1 as URL value",
			text: "fix PROJ-42",
			tickets: []config.Ticket{
				{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
			},
			want: []ticketLink{
				{Text: "PROJ-42", Href: "https://jira.example.com/browse/PROJ-42"},
			},
		},
		{
			name: "multiple matches from one ticket config",
			text: "fix PROJ-42 and PROJ-99",
			tickets: []config.Ticket{
				{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
			},
			want: []ticketLink{
				{Text: "PROJ-42", Href: "https://jira.example.com/browse/PROJ-42"},
				{Text: "PROJ-99", Href: "https://jira.example.com/browse/PROJ-99"},
			},
		},
		{
			name: "multiple ticket configs applied in order",
			text: "fix PROJ-42 see GH-7",
			tickets: []config.Ticket{
				{Pattern: `PROJ-(\d+)`, URL: "https://jira.example.com/browse/PROJ-{ticket}"},
				{Pattern: `GH-(\d+)`, URL: "https://github.com/acme/widget/issues/{ticket}"},
			},
			want: []ticketLink{
				{Text: "PROJ-42", Href: "https://jira.example.com/browse/PROJ-42"},
				{Text: "GH-7", Href: "https://github.com/acme/widget/issues/7"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, resolveTickets(tc.text, tc.tickets))
		})
	}
}

// ─── upperFirst ──────────────────────────────────────────────────────────────

func TestUpperFirst(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"add endpoint", "Add endpoint"},
		{"Add endpoint", "Add endpoint"},
		{"", ""},
		{"x", "X"},
		// multi-byte rune
		{"über cool", "Über cool"},
	}
	for _, tc := range tests {
		assert.Equal(t, tc.want, upperFirst(tc.in), "upperFirst(%q)", tc.in)
	}
}

// ─── renderPreamble (title/subtitle) ──────────────────────────────────────────

func TestRenderPreamble_ChangelogDefault(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, nil, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "# Changelog\n\n", out, "byte-identical to the former changelogHeader constant")
}

func TestRenderPreamble_ChangelogTitleOnly(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{"title": "# MyApp"}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "# MyApp\n\n", out)
}

func TestRenderPreamble_ChangelogTitleAndSubtitle(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{
		"title":    "# MyApp",
		"subtitle": "All notable changes.",
	}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "# MyApp\n\nAll notable changes.\n\n", out)
}

func TestRenderPreamble_ChangelogSubtitleOnly(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{
		"title":    "",
		"subtitle": "Just a subtitle.",
	}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "Just a subtitle.\n\n", out, "no leading blank line when title is explicitly nulled")
}

func TestRenderPreamble_ChangelogBothEmpty(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{"title": ""}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Empty(t, out, "an explicitly nulled title with no subtitle renders nothing, no stray blank line")
}

// TestRenderPreamble_FileWinsOverNullTitleSnippet proves a <driver>.template file redefining
// title wins over a null (empty-string) snippet override — the documented "a template file always
// wins outright" rule, previously inverted for title/subtitle by execPreambleBlock's Go-level
// short-circuit returning before templateFile was ever consulted.
func TestRenderPreamble_FileWinsOverNullTitleSnippet(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "custom.tmpl")
	require.NoError(t, os.WriteFile(file, []byte(`{{ define "title" }}# From File{{ end }}`), 0o644))

	out, err := renderPreamble(changelogTmpl, map[string]string{"title": ""}, file, tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "# From File\n\n", out, "the template file wins over a null title snippet override")
}

func TestRenderPreamble_HerautContextIsBareFields(t *testing.T) {
	out, err := renderPreamble(changelogTmpl, map[string]string{
		"title": "Changelog (heraut v{{ .Version }})",
	}, "", tplHeraut{Version: "0.59.0"})
	require.NoError(t, err)
	assert.Equal(t, "Changelog (heraut v0.59.0)\n\n", out,
		"title's root IS tplHeraut directly — .Version, not .Heraut.Version")
}

// ─── renderPostamble (document-level footer, ADR-0049) ────────────────────────

func TestRenderPostamble_ChangelogDefault(t *testing.T) {
	heraut := tplHeraut{Version: "1.2.3", URL: herautProjectURL, GeneratedAt: fixedDate1}
	out, err := renderPostamble(changelogTmpl, nil, "", heraut)
	require.NoError(t, err)
	assert.Equal(t, "_Generated by [heraut](https://github.com/adaouat/heraut) 1.2.3 at 12:00 on 2024-01-15._", out)
}

func TestRenderPostamble_Null(t *testing.T) {
	out, err := renderPostamble(changelogTmpl, map[string]string{"footer": ""}, "", tplHeraut{})
	require.NoError(t, err)
	assert.Empty(t, out, "a nulled footer snippet renders nothing")
}

func TestRenderPostamble_Override(t *testing.T) {
	out, err := renderPostamble(changelogTmpl, map[string]string{
		"footer": "-- built by heraut {{ .Version }}",
	}, "", tplHeraut{Version: "0.59.0"})
	require.NoError(t, err)
	assert.Equal(t, "-- built by heraut 0.59.0", out,
		"footer's root IS tplHeraut directly — .Version, not .Heraut.Version, same as title/subtitle")
}

// TestRenderPostamble_FileWinsOverNullFooterSnippet mirrors
// TestRenderPreamble_FileWinsOverNullTitleSnippet: a <driver>.template file redefining footer wins
// over a null (empty-string) snippet override.
func TestRenderPostamble_FileWinsOverNullFooterSnippet(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "custom.tmpl")
	require.NoError(t, os.WriteFile(file, []byte(`{{ define "footer" }}FROM FILE{{ end }}`), 0o644))

	out, err := renderPostamble(changelogTmpl, map[string]string{"footer": ""}, file, tplHeraut{})
	require.NoError(t, err)
	assert.Equal(t, "FROM FILE", out, "the template file wins over a null footer snippet override")
}

// TestRenderReleaseNotes_Contributors_Golden locks in the "New Contributors" block output — the
// one built-in path not covered by the other goldens (they pass nil contributors). It exercises
// the commit PR-suffix, the contributors block, and the stats block together.
func TestRenderReleaseNotes_Contributors_Golden(t *testing.T) {
	releaseDate := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	prevDate := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	prs := map[string]PullRequest{
		rc1.Hash: {Number: 7, URL: "https://github.com/acme/widget/pull/7", AuthorLogin: "alice", RefPrefix: "#"},
	}
	contribs := []Contributor{{
		Author:      Author{Name: "Alice", Email: "alice@example.com", Username: "alice"},
		IsFirstTime: true,
		PR:          &PullRequest{Number: 7, URL: "https://github.com/acme/widget/pull/7", AuthorLogin: "alice", RefPrefix: "#"},
	}}

	groups := fixtureGroups()
	overlayAuthorHandles(groups, map[string]string{rc1.Hash: "alice"})
	got, err := renderReleaseNotes(
		"v1.2.3", "v1.2.2", releaseDate,
		groups, githubLC, nil, prevDate, 3, prs, contribs, fixtureHeraut, nil, "",
	)
	require.NoError(t, err)

	const golden = "release_notes_contributors.golden"
	writeGolden(t, golden, got)
	want := readGolden(t, golden)
	assert.Equal(t, want, got)
}
