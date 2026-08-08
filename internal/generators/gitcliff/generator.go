package gitcliff

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// Remote-metadata policy values. Empty resolves to remoteOptional.
const (
	remoteOptional = "optional"
	remoteRequired = "required"
	remoteDisabled = "disabled"
)

// Mode selects which embedded TOML default to use.
type Mode int

const (
	ModeChangelog Mode = iota
	ModeReleaseNotes
)

// Generator implements port.Generator for git-cliff.
type Generator struct {
	runner   port.Runner
	cfg      *config.ContentDriver
	mode     Mode
	degraded bool
}

var _ port.Generator = (*Generator)(nil)

// New constructs a Generator for git-cliff.
func New(runner port.Runner, cfg *config.ContentDriver, mode Mode) *Generator {
	return &Generator{runner: runner, cfg: cfg, mode: mode}
}

// Degraded reports whether the most recent Generate/CheckCliff fell back to --offline
// because the remote metadata fetch failed under the "optional" policy. Callers
// (pipeline / check) use it to warn that PR author / number were omitted.
func (g *Generator) Degraded() bool { return g.degraded }

// Check verifies that git-cliff is available on PATH.
func (g *Generator) Check() error {
	_, _, err := g.runner.Run("git-cliff", "--version")
	if err != nil {
		return fmt.Errorf("git-cliff not found: %w", err)
	}
	return nil
}

// Validate checks that the user-specified config file exists (if any).
func (g *Generator) Validate() error {
	if g.cfg.Config == "" {
		return nil
	}
	if _, err := os.Stat(g.cfg.Config); err != nil {
		return fmt.Errorf("git-cliff config %q: %w", g.cfg.Config, err)
	}
	return nil
}

// Generate invokes git-cliff with the merged TOML config and returns stdout (release-notes
// mode) or empty string (changelog mode, where output goes to a file).
//
// When lc is non-nil, heraut-owned env vars (HERAUT_REMOTE_URL, HERAUT_PLATFORM) are
// injected into the git-cliff process so the template resolves links against that
// platform; when nil, git-cliff runs with the ambient environment and the template falls
// through to its CI-var detection (ADR-0021).
func (g *Generator) Generate(tag string, lc *port.LinkContext) (string, error) {
	cfgPath, cleanup, err := g.prepareConfig(lc)
	if err != nil {
		return "", err
	}
	defer cleanup()

	args := []string{"--config", cfgPath, "--tag", tag}

	// ModeChangelog: no range flag — git-cliff regenerates the full history so that
	// CHANGELOG.md always contains every release, not just the current one.
	//
	// ModeReleaseNotes: --latest — the tag is already pushed by the time release notes
	// are generated (step 6 of the release pipeline), so --unreleased would return
	// nothing. --latest returns the commits in the tag we just created.
	if g.mode == ModeReleaseNotes {
		args = append(args, "--latest")
	}

	if g.cfg.TagPattern != "" {
		args = append(args, "--tag-pattern", g.cfg.TagPattern)
	}

	if g.mode == ModeChangelog && g.cfg.Output != "" {
		args = append(args, "--output", g.cfg.Output)
	}

	stdout, err := g.runCliff(args, lc)
	if err != nil {
		return "", fmt.Errorf("git-cliff: %w", err)
	}
	return stdout, nil
}

// runCliff runs git-cliff applying the remote-metadata policy:
//   - disabled: always pass --offline (never reach the platform API).
//   - required: never pass --offline; a remote-fetch failure is fatal — but only when a fetch is
//     actually attempted. With no forge resolved (lc nil / no owner-repo), injectRemote adds no
//     [remote.*] section, so git-cliff has nothing to fetch and exits cleanly with unenriched
//     output and no error: required does not assert that a forge exists (T174, documented
//     divergence from native — see docs/specs/05-generators-and-platforms.md "Auto-detection and
//     self-hosted hosts").
//   - optional (default): try online; on any failure retry with --offline. If the offline
//     retry succeeds the run is marked degraded; if it also fails the original (online)
//     error is returned so a genuine config error still surfaces.
func (g *Generator) runCliff(args []string, lc *port.LinkContext) (string, error) {
	switch g.cfg.RemoteMetadata {
	case remoteDisabled:
		return g.exec(appendOffline(args), lc)
	case remoteRequired:
		return g.exec(args, lc)
	default: // remoteOptional
		stdout, err := g.exec(args, lc)
		if err == nil {
			return stdout, nil
		}
		offlineOut, offlineErr := g.exec(appendOffline(args), lc)
		if offlineErr != nil {
			return "", err
		}
		g.degraded = true
		return offlineOut, nil
	}
}

// appendOffline returns args with --offline appended, without mutating the input.
func appendOffline(args []string) []string {
	return append(slices.Clone(args), "--offline")
}

func (g *Generator) exec(args []string, lc *port.LinkContext) (string, error) {
	if lc != nil {
		stdout, _, err := g.runner.RunEnv(linkEnv(lc), "git-cliff", args...)
		return stdout, err
	}
	stdout, _, err := g.runner.Run("git-cliff", args...)
	return stdout, err
}

// linkEnv translates a LinkContext into the heraut-owned env vars the embedded git-cliff
// template reads via get_env: HERAUT_REMOTE_URL (the repository root URL) and
// HERAUT_PLATFORM (github|gitlab|azure_devops, for the PR vs MR link shape). The embedded
// template prefers these over the ambient CI-var chain.
func linkEnv(lc *port.LinkContext) []string {
	if lc.Platform == "azure_devops" {
		return azureDevOpsLinkEnv(lc)
	}

	remote := strings.TrimRight(lc.BaseURL, "/")
	if lc.Owner != "" {
		remote += "/" + lc.Owner
	}
	if lc.Repo != "" {
		remote += "/" + lc.Repo
	}

	// GitLab routes everything below the project under /-/ (e.g. /-/merge_requests/);
	// GitHub does not and uses /pull/. The label glyph differs too (! vs #). This is the
	// only per-platform path knowledge — kept here in Go (table-tested) so the templates
	// stay branch-free (ADR-0022).
	infix, prPath, label := "", "pull", "#"
	if lc.Platform == "gitlab" {
		infix, prPath, label = "/-", "merge_requests", "!"
	}

	env := []string{
		"HERAUT_REMOTE_URL=" + remote,
		"HERAUT_PLATFORM=" + lc.Platform,
		"HERAUT_COMMIT_URL=" + remote + infix + "/commit/",
		"HERAUT_PR_URL=" + remote + infix + "/" + prPath + "/",
		"HERAUT_PR_LABEL=" + label,
		"HERAUT_COMPARE_URL=" + remote + infix + "/compare/",
	}

	// Inject the platform token as the standard var git-cliff expects (GITHUB_TOKEN or
	// GITLAB_TOKEN), but only when not already set — avoids overriding a token the user
	// explicitly configured in their CI environment.
	if lc.Token != "" {
		tokenVar := "GITHUB_TOKEN"
		if lc.Platform == "gitlab" {
			tokenVar = "GITLAB_TOKEN"
		}
		if os.Getenv(tokenVar) == "" {
			env = append(env, tokenVar+"="+lc.Token)
		}
	}

	// Inject the repo coordinates git-cliff's [remote.*] feature reads (GITHUB_REPO or
	// GITLAB_REPO in owner/repo format). Only inject when owner and repo are known from
	// the platform context; ambient contexts (Owner/Repo empty) are skipped. Guards
	// against overriding a value the user set explicitly in their CI environment.
	if lc.Owner != "" && lc.Repo != "" {
		repoVar := "GITHUB_REPO"
		if lc.Platform == "gitlab" {
			repoVar = "GITLAB_REPO"
		}
		if os.Getenv(repoVar) == "" {
			env = append(env, repoVar+"="+lc.Owner+"/"+lc.Repo)
		}
	}

	return env
}

// azureDevOpsTokenEnvVar and azureDevOpsRepoEnvVar are the env vars git-cliff's
// azure_devops remote integration reads, mirroring GITHUB_TOKEN/GITHUB_REPO and
// GITLAB_TOKEN/GITLAB_REPO (ADR-0026).
const (
	azureDevOpsTokenEnvVar = "AZURE_DEVOPS_TOKEN"
	azureDevOpsRepoEnvVar  = "AZURE_DEVOPS_REPO"
)

// urlPathSegments percent-encodes each "/"-delimited segment of s independently,
// preserving the literal "/" separators. Azure DevOps organization/project/repository
// names may contain spaces and other characters that are valid identifiers but invalid
// unescaped in a URL path segment (e.g. "sub group" -> "sub%20group").
func urlPathSegments(s string) string {
	parts := strings.Split(s, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// azureDevOpsLinkEnv builds the heraut-owned env vars for an Azure DevOps remote
// (ADR-0026). Azure Repos URLs are structurally different from GitHub/GitLab: the
// repository root inserts /_git/ between the project and repository segments
// (https://dev.azure.com/{org}/{project}/_git/{repo}), and the compare URL is
// query-string based with two separately-prefixed refs
// (?baseVersion=GT{old}&targetVersion=GT{new}) rather than a single "{prefix}{old}..{new}"
// path. HERAUT_COMPARE_URL carries the prefix only; HERAUT_COMPARE_URL_MIDDLE (T115 /
// ADR-0022 update) carries the rest — the embedded template substitutes both around
// {previous.version}/{version}. GitHub/GitLab never set MIDDLE, so the template's default
// ("..") reproduces their exact pre-T115 output.
func azureDevOpsLinkEnv(lc *port.LinkContext) []string {
	remote := strings.TrimRight(lc.BaseURL, "/")
	if lc.Owner != "" {
		remote += "/" + urlPathSegments(lc.Owner)
	}
	repoRoot := remote
	if lc.Repo != "" {
		repoRoot += "/_git/" + url.PathEscape(lc.Repo)
	}

	env := []string{
		"HERAUT_REMOTE_URL=" + repoRoot,
		"HERAUT_PLATFORM=" + lc.Platform,
		"HERAUT_COMMIT_URL=" + repoRoot + "/commit/",
		"HERAUT_PR_URL=" + repoRoot + "/pullrequest/",
		"HERAUT_PR_LABEL=#",
		"HERAUT_COMPARE_URL=" + repoRoot + "/branchCompare?baseVersion=GT",
		"HERAUT_COMPARE_URL_MIDDLE=&targetVersion=GT",
	}

	if lc.Token != "" && os.Getenv(azureDevOpsTokenEnvVar) == "" {
		env = append(env, azureDevOpsTokenEnvVar+"="+lc.Token)
	}

	if lc.Owner != "" && lc.Repo != "" && os.Getenv(azureDevOpsRepoEnvVar) == "" {
		env = append(env, azureDevOpsRepoEnvVar+"="+lc.Owner+"/"+lc.Repo)
	}

	return env
}

// injectRemote appends [remote.github] or [remote.gitlab] to the merged TOML when the
// link context carries owner and repo, so git-cliff can fetch PR/MR metadata without
// requiring a custom config. Skipped when lc is nil, owner/repo are empty (ambient
// context), or the user's override already declares [remote.<platform>].
func injectRemote(merged string, lc *port.LinkContext) (string, error) {
	if lc == nil || lc.Owner == "" || lc.Repo == "" {
		return merged, nil
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(merged), &doc); err != nil {
		return "", fmt.Errorf("parsing merged TOML for remote injection: %w", err)
	}
	remote, _ := doc["remote"].(map[string]any)
	if remote != nil {
		if _, exists := remote[lc.Platform]; exists {
			return merged, nil
		}
	}
	if remote == nil {
		remote = make(map[string]any)
		doc["remote"] = remote
	}
	remote[lc.Platform] = map[string]any{"owner": lc.Owner, "repo": lc.Repo}
	out, err := toml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling TOML after remote injection: %w", err)
	}
	return string(out), nil
}

// EffectiveChangelogConfig returns the merged TOML for the changelog variant.
func (g *Generator) EffectiveChangelogConfig() (string, error) {
	return g.effectiveConfig(embeddedChangelog)
}

// EffectiveReleaseNotesConfig returns the merged TOML for the release-notes variant.
func (g *Generator) EffectiveReleaseNotesConfig() (string, error) {
	return g.effectiveConfig(embeddedReleaseNotes)
}

func (g *Generator) effectiveConfig(base string) (string, error) {
	override := ""
	if g.cfg.Config != "" {
		data, err := os.ReadFile(g.cfg.Config)
		if err != nil {
			return "", fmt.Errorf("reading git-cliff config %q: %w", g.cfg.Config, err)
		}
		override = string(data)
	}
	merged, err := MergeTOML(base, override)
	if err != nil {
		return "", err
	}
	withHeading, err := injectHeadingPostprocessor(merged, g.cfg.HeadingVersionPattern)
	if err != nil {
		return "", err
	}
	return injectLinkParsers(withHeading, g.cfg.Tickets)
}

// injectLinkParsers appends one git-cliff link_parser per ticket to the [git] table of the
// merged TOML: { pattern = <P>, href = <url with {ticket}→$1> }. <P> is the user pattern
// wrapped in a capture group only when it has none, so $1 is the URL value; the link label
// defaults to the full match (git-cliff's text default). Existing user link_parsers are
// preserved — heraut's entries are appended after them.
func injectLinkParsers(merged string, tickets []config.Ticket) (string, error) {
	if len(tickets) == 0 {
		return merged, nil
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(merged), &doc); err != nil {
		return "", fmt.Errorf("parsing merged TOML for link_parsers injection: %w", err)
	}
	git, _ := doc["git"].(map[string]any)
	if git == nil {
		git = make(map[string]any)
		doc["git"] = git
	}
	entries := make([]any, 0, len(tickets))
	for _, t := range tickets {
		pattern := t.Pattern
		re, err := regexp.Compile(pattern)
		if err != nil {
			return "", fmt.Errorf("ticket pattern %q: %w", t.Pattern, err)
		}
		if re.NumSubexp() == 0 {
			pattern = "(" + pattern + ")"
		}
		href := strings.ReplaceAll(t.URL, "{ticket}", "$1")
		entries = append(entries, map[string]any{"pattern": pattern, "href": href})
	}
	switch existing := git["link_parsers"].(type) {
	case []any:
		git["link_parsers"] = append(existing, entries...)
	default:
		git["link_parsers"] = entries
	}
	out, err := toml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling TOML after link_parsers injection: %w", err)
	}
	return string(out), nil
}

// injectHeadingPostprocessor prepends a postprocessor entry (derived from the effective
// tag_format) to the [changelog] postprocessors array in the merged TOML. This strips the
// env prefix/suffix and build ID from version headings at render time. When pattern is
// empty, merged is returned unchanged.
func injectHeadingPostprocessor(merged, pattern string) (string, error) {
	if pattern == "" {
		return merged, nil
	}
	var doc map[string]any
	if err := toml.Unmarshal([]byte(merged), &doc); err != nil {
		return "", fmt.Errorf("parsing merged TOML for postprocessor injection: %w", err)
	}

	changelog, _ := doc["changelog"].(map[string]any)
	if changelog == nil {
		changelog = make(map[string]any)
		doc["changelog"] = changelog
	}

	entry := map[string]any{"pattern": pattern, "replace": "[$1]"}
	switch existing := changelog["postprocessors"].(type) {
	case []any:
		changelog["postprocessors"] = append([]any{entry}, existing...)
	default:
		changelog["postprocessors"] = []any{entry}
	}

	out, err := toml.Marshal(doc)
	if err != nil {
		return "", fmt.Errorf("marshalling TOML after postprocessor injection: %w", err)
	}
	return string(out), nil
}

// CheckCliff runs git-cliff --context --no-exec against the effective merged config.
// Called by `heraut check cliff`.
func (g *Generator) CheckCliff() error {
	cfgPath, cleanup, err := g.prepareConfig(nil)
	if err != nil {
		return err
	}
	defer cleanup()
	args := []string{"--context", "--no-exec", "--config", cfgPath}
	if _, err := g.runCliff(args, nil); err != nil {
		return fmt.Errorf("git-cliff rejected config: %w", err)
	}
	return nil
}

// prepareConfig writes the effective merged TOML to a temp file and returns its path
// plus a cleanup function. The caller must call cleanup() to remove the temp file.
// When lc is non-nil and carries owner/repo, injectRemote appends [remote.<platform>]
// to the TOML so git-cliff's PR/MR metadata fetching works without a custom config.
func (g *Generator) prepareConfig(lc *port.LinkContext) (string, func(), error) {
	var base string
	if g.mode == ModeChangelog {
		base = embeddedChangelog
	} else {
		base = embeddedReleaseNotes
	}

	merged, err := g.effectiveConfig(base)
	if err != nil {
		return "", func() {}, err
	}

	merged, err = injectRemote(merged, lc)
	if err != nil {
		return "", func() {}, err
	}

	tmp, err := os.CreateTemp("", "heraut-cliff-*.toml")
	if err != nil {
		return "", func() {}, fmt.Errorf("creating temp cliff config: %w", err)
	}
	if _, err := tmp.WriteString(merged); err != nil {
		_ = os.Remove(tmp.Name())
		return "", func() {}, fmt.Errorf("writing temp cliff config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmp.Name())
		return "", func() {}, fmt.Errorf("closing temp cliff config: %w", err)
	}

	path := tmp.Name()
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}
