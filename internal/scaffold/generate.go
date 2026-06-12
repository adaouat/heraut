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
			}
		}
	}

	if a.ChangelogGenerator != "" {
		output := a.ChangelogOutput
		if output == "" {
			output = "CHANGELOG.md"
		}
		cfg.Changelog = &config.ContentDriver{
			Generator: a.ChangelogGenerator,
			Output:    output,
		}
	}

	hasNotes := a.NotesGenerator != ""
	hasPlatforms := len(a.Platforms) > 0
	if hasNotes || hasPlatforms {
		cfg.Release = &config.Release{}
		if hasNotes {
			cfg.Release.Notes = &config.ContentDriver{
				Generator: a.NotesGenerator,
			}
		}
		platformTypeCount := make(map[string]int)
		for _, p := range a.Platforms {
			platformTypeCount[p.Type]++
			name := p.Type
			if n := platformTypeCount[p.Type]; n > 1 {
				name = fmt.Sprintf("%s-%d", p.Type, n)
			}
			plat := config.Platform{
				Name:       name,
				Type:       p.Type,
				Repository: p.Repository,
				Project:    p.Project,
				TokenEnv:   p.TokenEnv,
			}
			cfg.Release.Platforms = append(cfg.Release.Platforms, plat)
		}
	}

	return cfg
}
