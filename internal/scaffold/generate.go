package scaffold

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/adaouat/heraut/internal/config"
)

const schemaBase = "https://raw.githubusercontent.com/adaouat/heraut/"
const schemaFile = "/schema.json"

func buildSchemaURL(version string) string {
	if version == "" || version == "dev" {
		return schemaBase + "main" + schemaFile
	}
	return schemaBase + version + schemaFile
}

// GenerateYAML converts answers to a .heraut.yml string with a yaml-language-server header.
// version is the heraut build version (e.g. "v1.2.3"); empty or "dev" falls back to main.
func GenerateYAML(a Answers, version string) (string, error) {
	cfg := answersToConfig(a)

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(cfg); err != nil {
		return "", fmt.Errorf("generating YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return "", fmt.Errorf("finalizing YAML: %w", err)
	}

	header := "# yaml-language-server: $schema=" + buildSchemaURL(version) + "\n\n"
	return header + buf.String(), nil
}

func answersToConfig(a Answers) config.Config {
	cfg := config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: a.Strategy,
		},
	}
	if len(a.Tickets) > 0 || a.EnrichmentPolicy != "" {
		cfg.Commits = &config.Commits{Tickets: a.Tickets, EnrichmentPolicy: a.EnrichmentPolicy}
	}

	// Always write prefix — even when empty — so the resolver does not fall
	// back to the "v" default (nil = unset = use default; &"" = no prefix).
	prefix := a.TagPrefix
	cfg.Versioning.TagPrefix = &prefix

	if a.Format != "" {
		cfg.Versioning.Format = a.Format
	}

	if a.Sprint > 0 {
		cfg.Versioning.Sprint = a.Sprint
	}

	if len(a.Environments) > 0 {
		cfg.Versioning.TagFormat = a.TagFormat
		cfg.Environments = make(map[string]config.Environment, len(a.Environments))
		for _, e := range a.Environments {
			cfg.Environments[e.Name] = config.Environment{
				Bump:             e.Bump,
				TagFormat:        e.TagFormat,
				Source:           e.Source,
				Branch:           e.Branch,
				DisableChangelog: e.DisableChangelog,
				DisableRelease:   e.DisableNotes,
				Changelog:        e.Changelog,
				Release:          e.Release,
			}
		}
	}

	if a.EnableChangelog {
		output := a.ChangelogOutput
		if output == "" {
			output = "CHANGELOG.md"
		}
		cfg.Changelog = &config.ContentDriver{
			Output: output,
		}
	}

	hasNotes := a.EnableReleaseNotes
	hasPlatforms := len(a.Platforms) > 0
	hasAssets := len(a.Assets) > 0
	if hasNotes || hasPlatforms || hasAssets {
		cfg.Release = &config.Release{Assets: a.Assets}
		if hasNotes {
			cfg.Release.Notes = &config.ContentDriver{}
		}
		names := platformDisplayNames(a.Platforms)
		for i, p := range a.Platforms {
			name := names[i]
			cfg.Forges = append(cfg.Forges, config.Forge{
				Name:       name,
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
				APIMode:    p.APIMode,
				BaseURL:    p.BaseURL,
			})
			cfg.Release.Targets = append(cfg.Release.Targets, config.Target{
				Forge:      name,
				Draft:      p.Draft,
				Prerelease: p.Prerelease,
			})
		}
		// commits.enrichment_forge is required once more than one forge is configured
		// (validateForges). The wizard (runEnrichmentWizard) now asks explicitly whenever
		// there are 2+ platforms, so a.EnrichmentForge is normally already set correctly by the
		// time this runs. This fallback exists for direct/non-wizard callers of answersToConfig
		// (e.g. a hand-built Answers, or a future --defaults preset with multiple platforms):
		// preserve an existing choice when it still names one of the rebuilt forges, else fall
		// back to the first configured forge.
		if len(cfg.Forges) > 1 {
			if cfg.Commits == nil {
				cfg.Commits = &config.Commits{}
			}
			enrichmentForge := a.EnrichmentForge
			if !forgeNameExists(cfg.Forges, enrichmentForge) {
				enrichmentForge = cfg.Forges[0].Name
			}
			cfg.Commits.EnrichmentForge = enrichmentForge
		}
	}

	return cfg
}

// platformDisplayNames returns the forges[].name each platform in ps will be assigned once
// generated — type-based, deduplicated with a "-N" suffix for repeats. answersToConfig and the
// wizard's enrichment-forge prompt both need the exact same computation so the names offered to
// the user match what GenerateYAML actually writes.
func platformDisplayNames(ps []PlatformAnswer) []string {
	names := make([]string, len(ps))
	typeCount := make(map[string]int)
	for i, p := range ps {
		typeCount[p.Type]++
		name := p.Name
		if name == "" {
			name = p.Type
			if n := typeCount[p.Type]; n > 1 {
				name = fmt.Sprintf("%s-%d", p.Type, n)
			}
		}
		names[i] = name
	}
	return names
}

// forgeNameExists reports whether name matches one of forges[].name.
func forgeNameExists(forges []config.Forge, name string) bool {
	if name == "" {
		return false
	}
	for _, f := range forges {
		if f.Name == name {
			return true
		}
	}
	return false
}
