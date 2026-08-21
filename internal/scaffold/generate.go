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
				DisableNotes:     e.DisableNotes,
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
		platformTypeCount := make(map[string]int)
		for _, p := range a.Platforms {
			platformTypeCount[p.Type]++
			name := p.Name
			if name == "" {
				name = p.Type
				if n := platformTypeCount[p.Type]; n > 1 {
					name = fmt.Sprintf("%s-%d", p.Type, n)
				}
			}
			cfg.Forges = append(cfg.Forges, config.Forge{
				Name:       name,
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
				BaseURL:    p.BaseURL,
			})
			cfg.Release.Targets = append(cfg.Release.Targets, config.Target{
				Forge:      name,
				Draft:      p.Draft,
				Prerelease: p.Prerelease,
			})
		}
		// commits.enrichment_forge is required once more than one forge is configured
		// (validateForges) — the wizard has no forge-selection question yet (that redesign is
		// T164/P4). Preserve an existing choice carried through Answers.EnrichmentForge (I7:
		// silently repointing it at the first forge on every re-run of `heraut init` changed
		// which host PR/MR metadata comes from with no warning); only fall back to the first
		// configured forge — the deliberate, unambiguous stopgap default — when there is no
		// prior choice, or it no longer names one of the rebuilt forges.
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
