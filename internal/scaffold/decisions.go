package scaffold

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/versioning/calver"
)

// detectPlatform pre-fills a new platform's type and project/repository path: CI
// environment/known-host detection (via internal/forge) identifies the type when possible; the
// project/repository path falls back to parseRemoteProject's any-host parsing when forge
// detection doesn't apply (self-hosted instances, no CI markers) or names a type the wizard's
// platform Select doesn't offer (only "github"/"gitlab" are wizard-supported today).
func detectPlatform(getenv func(string) string, gitOrigin string) (typ, projectOrRepo string) {
	if t, p, ok := forge.DetectForWizard(getenv, gitOrigin); ok && (t == "github" || t == "gitlab") {
		if p == "" {
			p = parseRemoteProject(gitOrigin)
		}
		return t, p
	}
	return "", parseRemoteProject(gitOrigin)
}

// parseRemoteProject extracts "namespace/project" (or "owner/repo") from a git
// remote URL. Handles SSH (git@host:path.git), ssh:// and HTTPS schemes.
func parseRemoteProject(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	// Strip any scheme prefix (https://, ssh://, git+ssh://, …)
	if i := strings.Index(rawURL, "://"); i != -1 {
		rawURL = rawURL[i+3:]
		// Now looks like "host/path.git" — strip the host.
		if i := strings.Index(rawURL, "/"); i != -1 {
			rawURL = rawURL[i+1:]
		} else {
			return ""
		}
	} else if i := strings.Index(rawURL, ":"); i != -1 {
		// SCP-style SSH: git@host:path.git — everything after the colon is the path.
		rawURL = rawURL[i+1:]
	}
	return strings.TrimSuffix(rawURL, ".git")
}

// platformTokenOptions returns the known token env var names for a platform,
// in display order. The first entry is the recommended default.
func platformTokenOptions(platformType string) []string {
	switch platformType {
	case "github":
		return []string{config.EnvGitHubCLIToken}
	case "gitlab":
		return []string{config.EnvGitLabToken, config.EnvGitLabJobToken}
	default:
		return nil
	}
}

// resolveTokenChoice maps an existing token env var to a wizard select choice
// and optional custom value. When existing is empty the first known token is
// returned as the default. Unrecognised values map to ("custom", existing).
func resolveTokenChoice(platformType, existing string) (choice, custom string) {
	opts := platformTokenOptions(platformType)
	if existing == "" {
		if len(opts) > 0 {
			return opts[0], ""
		}
		return "custom", ""
	}
	if slices.Contains(opts, existing) {
		return existing, ""
	}
	return "custom", existing
}

// hideAPIMode reports whether the api_mode prompt should stay hidden: for any non-GitLab
// platform, or when CI_JOB_TOKEN is the chosen token — GitLab's GraphQL API structurally rejects
// job tokens, so offering graphql there would let the wizard produce a config guaranteed to fail
// at enrichment time.
func hideAPIMode(platformType, tokenChoice string) bool {
	return platformType != "gitlab" || tokenChoice == config.EnvGitLabJobToken
}

// matchPlatformSnapshot looks up the previously-configured platform of the given type that
// corresponds to the platform currently being built, by type-scoped position: the n-th platform of
// a type appended so far in this wizard run (rebuiltSoFar) matches the n-th snapshot entry of that
// type. Called once per loop iteration in runPlatformWizard, right after Step 1 sets p.Type — early
// enough that Step 3's token/api_mode prompts can pre-fill from the match, unlike a post-loop batch
// pass which would see every prompt already rendered. The same call also supplies the passthrough
// fields (Name, BaseURL, Draft), which aren't read by any prompt but are matched here
// too so every field on a given platform always comes from the same original snapshot entry —
// splitting this into two independent matching algorithms (one for TokenEnv/APIMode, one for the
// passthrough fields) let them silently drift apart if either changed without the other; unifying
// them into one lookup closes that risk structurally instead of by convention (T204).
func matchPlatformSnapshot(snapshot, rebuiltSoFar []PlatformAnswer, platformType string) (PlatformAnswer, bool) {
	index := 0
	for _, p := range rebuiltSoFar {
		if p.Type == platformType {
			index++
		}
	}
	n := -1
	for _, p := range snapshot {
		if p.Type != platformType {
			continue
		}
		n++
		if n == index {
			return p, true
		}
	}
	return PlatformAnswer{}, false
}

// validateEnvName rejects an empty name and one already used by an earlier environment in this
// wizard run (T259): generate.go assigns environments into a map keyed by name
// (cfg.Environments[e.Name] = ...), so a repeated name would otherwise silently overwrite the
// earlier environment with no error at any stage, not even config.Validate.
func validateEnvName(existing []EnvAnswer, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("environment name is required")
	}
	for _, e := range existing {
		if e.Name == name {
			return fmt.Errorf("environment %q is already defined", name)
		}
	}
	return nil
}

// validateEnvSource mirrors validatePerEnv's "source must reference an existing environment, not
// itself" rule (internal/config/validator.go) for heraut init's live field validation (T259).
// Scoped to what the wizard can actually know mid-run: existing holds only the environments
// defined earlier in this loop, so a source naming one not yet added is rejected here even though
// it might be added later — the wizard's one-at-a-time flow expects sources defined first. Empty
// is always valid: validatePerEnv accepts it as "auto-detect the sole auto environment," a
// cross-environment count this per-field validator doesn't attempt to replicate.
func validateEnvSource(existing []EnvAnswer, currentName, source string) error {
	if source == "" {
		return nil
	}
	if source == currentName {
		return fmt.Errorf("environment cannot promote from itself")
	}
	for _, e := range existing {
		if e.Name == source {
			return nil
		}
	}
	return fmt.Errorf("environment %q does not exist yet — define it before this one, or leave empty to auto-detect", source)
}

// matchEnvSnapshot carries passthrough fields (Changelog, Release) from the pre-wizard
// snapshot to the rebuilt environment list using name-based matching. Rebuilt entries
// with no name match in original are returned unchanged.
func matchEnvSnapshot(original, rebuilt []EnvAnswer) []EnvAnswer {
	byName := make(map[string]EnvAnswer, len(original))
	for _, e := range original {
		byName[e.Name] = e
	}
	result := make([]EnvAnswer, len(rebuilt))
	for i, e := range rebuilt {
		if orig, ok := byName[e.Name]; ok {
			e.Changelog = orig.Changelog
			e.Release = orig.Release
		}
		result[i] = e
	}
	return result
}

// shouldPromptEnrichmentForge reports whether the enrichment-forge select should be shown: only
// when 2+ platforms are configured, since with 0 or 1 the choice is unambiguous and the field is
// left for runtime auto-detection or the single configured forge to resolve.
func shouldPromptEnrichmentForge(platforms []PlatformAnswer) bool {
	return len(platforms) >= 2
}

// resolveFormatChoice maps an existing format string to a preset choice key and
// optional custom value. Returns the first preset when format is empty.
func resolveFormatChoice(format string) (choice string, custom string) {
	if format == "" {
		return calverPresets[0].format, ""
	}
	for _, p := range calverPresets {
		if p.format != "custom" && p.format == format {
			return format, ""
		}
	}
	return "custom", format
}

// suspiciousTokenRE finds uppercase-only substrings of 2+ chars in a literal —
// these are likely mistyped token names (e.g. "YYY", "OOOO").
var suspiciousTokenRE = regexp.MustCompile(`[A-Z]{2,}`)

// ValidateCalVerFormat validates a CalVer format string for use in .heraut.yml.
func ValidateCalVerFormat(format string) error {
	if strings.TrimSpace(format) == "" {
		return fmt.Errorf("format is required")
	}
	tokens, err := calver.ParseFormat(format)
	if err != nil {
		return err
	}
	hasCal := false
	for _, t := range tokens {
		switch t.Kind {
		case calver.KindYYYY, calver.KindMM, calver.KindDD,
			calver.KindWW, calver.KindQQ, calver.KindSS, calver.KindSPRINT:
			hasCal = true
		}
	}
	if !hasCal {
		return fmt.Errorf("format must contain at least one calendar token (YYYY, MM, DD, WW, QQ, SS, or SPRINT)")
	}
	for _, t := range tokens {
		if t.Kind == calver.KindLiteral {
			if m := suspiciousTokenRE.FindString(t.Literal); m != "" {
				return fmt.Errorf("unknown token %q; valid tokens: YYYY MM DD WW QQ SS SPRINT PATCH", m)
			}
		}
	}
	return nil
}

// finishPlatformToken turns the token and API-mode answers into the stored values: a custom token
// is trimmed, and the API mode is kept only where the prompt was shown (hideAPIMode), so a hidden
// prompt never leaves a stale value behind.
func finishPlatformToken(platformType, tokenChoice, customToken, apiMode string) (tokenEnv, mode string) {
	tokenEnv = tokenChoice
	if tokenChoice == "custom" {
		tokenEnv = strings.TrimSpace(customToken)
	}
	if hideAPIMode(platformType, tokenChoice) {
		return tokenEnv, ""
	}
	return tokenEnv, apiMode
}

// enrichmentForgeChoice keeps current when it still names a configured platform and otherwise
// falls back to the first one, so a removed or renamed platform never leaves a dangling choice.
func enrichmentForgeChoice(names []string, current string) string {
	if slices.Contains(names, current) {
		return current
	}
	return names[0]
}

// enrichmentPolicyOrDefault returns the policy already chosen, or "optional" when there is none.
func enrichmentPolicyOrDefault(current string) string {
	if current == "" {
		return "optional"
	}
	return current
}

// applyPlatformSnapshot re-seeds the fields the wizard carries through from the platform this entry
// was rebuilt from (T108). Type, project and repository are asked fresh, so they are not copied.
func applyPlatformSnapshot(p *PlatformAnswer, orig PlatformAnswer) {
	p.Name = orig.Name
	p.BaseURL = orig.BaseURL
	p.Draft = orig.Draft
	p.TokenEnv = orig.TokenEnv
	p.APIMode = orig.APIMode
}

// prefillPlatformLocation fills the repository (GitHub) or project (GitLab) with the detected
// value when the entry has none yet.
func prefillPlatformLocation(p *PlatformAnswer, detected string) {
	switch p.Type {
	case "github":
		if p.Repository == "" {
			p.Repository = detected
		}
	case "gitlab":
		if p.Project == "" {
			p.Project = detected
		}
	}
}

// tokenSelectOptions is the token env var choices offered for a platform: its known names, then
// "custom".
func tokenSelectOptions(platformType string) []string {
	return append(platformTokenOptions(platformType), "custom")
}

// apiModeOrDefault is the API mode the form starts on: the existing one, else "rest".
func apiModeOrDefault(mode string) string {
	if mode == "" {
		return "rest"
	}
	return mode
}

func requireGitHubRepository(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("repository is required for GitHub (e.g. org/repo)")
	}
	return nil
}

func requireTokenEnv(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("token env var is required")
	}
	return nil
}
