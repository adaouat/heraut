package app

import (
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/forge"
	"github.com/adaouat/heraut/internal/platforms/github"
	"github.com/adaouat/heraut/internal/platforms/gitlab"
	"github.com/adaouat/heraut/internal/port"
)

func buildGitHubPlatform(runner port.Runner, cfg *config.Platform) (port.Platform, error) {
	return github.New(runner, cfg), nil
}

func buildGitLabPlatform(runner port.Runner, cfg *config.Platform) (port.Platform, error) {
	return gitlab.New(runner, cfg), nil
}

// platformBuilders maps a resolved forge type to its publish-driver constructor — the one source
// of truth for which forge types support publishing (T221). buildPlatform dispatches from it
// directly; supportsPublish (consulted by synthesizeDefaultTarget and, transitively,
// HasResolvablePublishTarget) checks membership in the same map, so the two can never drift apart.
// Azure DevOps has no entry and never will: it has no equivalent of a GitHub/GitLab Release (a
// tag-attached page for notes + downloadable assets) to build a driver against — Azure Pipelines'
// own "Releases" is an unrelated multi-stage deployment-orchestration feature, not a publishable
// artifact.
var platformBuilders = map[string]func(port.Runner, *config.Platform) (port.Platform, error){
	"github": buildGitHubPlatform,
	"gitlab": buildGitLabPlatform,
}

// supportsPublish reports whether platformType has a publish driver.
func supportsPublish(platformType string) bool {
	_, ok := platformBuilders[strings.ToLower(platformType)]
	return ok
}

// platformConfigFromTarget builds the config.Platform an existing platform driver accepts from a
// resolved forge identity plus a target's publish options. The drivers are unchanged (ADR-0043 P3):
// the identity supplies host and project/repository, so publishing inherits CI/git auto-detection,
// while the token still resolves via TokenEnv because the drivers own their auth environment.
//
// f.Name labels every user-facing publish string (Platform.Name(): "Publish to <name>",
// "[dry-run] would publish to <name>", error wrapping, the release URL line, and heraut check's
// Platforms row). Zero-config publishing (no forges: entry) leaves f.Name empty, so it falls back
// to the resolved forge type — never blank.
func platformConfigFromTarget(t config.Target, f config.Forge, id port.ForgeIdentity) config.Platform {
	name := f.Name
	if name == "" {
		name = id.Type
	}
	cfg := config.Platform{
		Name:       name,
		Type:       id.Type,
		BaseURL:    id.Host,
		TokenEnv:   f.TokenEnv,
		Draft:      t.Draft,
		Prerelease: t.Prerelease,
		Assets:     t.Assets,
	}
	if id.Type == "github" {
		cfg.Repository = id.Project
	} else {
		cfg.Project = id.Project
		cfg.Repository = id.Repository
	}
	return cfg
}

// synthesizeDefaultTarget decides what an empty release.targets list means: zero-config
// publishing (one implicit default target for the resolved forge) when a forge resolves to a type
// with a publish driver, or no publish target at all otherwise. release: presence always means
// "publish" (T216, release atomicity) — there is no longer a config-expressible "notes only" state
// to protect against — but a resolved forge with no publish driver (T221 — e.g. azure_devops) is
// not a usable publish target either, so it must not be synthesized as one: doing so would carry
// the pipeline all the way to buildPlatform's generic "unsupported platform" error instead of
// behaving like "no forge resolved," which is what heraut release's own preflight check
// (HasResolvablePublishTarget) already reports clearly.
func synthesizeDefaultTarget(resolved forge.Resolved) []config.Target {
	for _, f := range resolved.Forges {
		if supportsPublish(f.Type) {
			return []config.Target{{}}
		}
	}
	return nil
}
