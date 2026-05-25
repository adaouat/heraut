package scaffold

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/adaouat/heraut/internal/config"
)

const schemaURL = "https://raw.githubusercontent.com/adaouat/heraut/main/schema.json"

// GenerateYAML converts answers to a .heraut.yml string with a yaml-language-server header.
func GenerateYAML(a Answers) (string, error) {
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

	header := "# yaml-language-server: $schema=" + schemaURL + "\n\n"
	return header + buf.String(), nil
}

func answersToConfig(a Answers) config.Config {
	cfg := config.Config{
		Version: "1",
		Versioning: config.Versioning{
			Strategy: a.Strategy,
		},
	}

	if a.Prefix != "" {
		prefix := a.Prefix
		cfg.Versioning.Prefix = &prefix
	}

	if a.Format != "" {
		cfg.Versioning.Format = a.Format
	}

	if len(a.Environments) > 0 {
		cfg.Versioning.TagFormat = a.TagFormat
		cfg.Versioning.Environments = make(map[string]config.EnvVersioning, len(a.Environments))
		for _, e := range a.Environments {
			cfg.Versioning.Environments[e.Name] = config.EnvVersioning{
				Bump:      e.Bump,
				TagFormat: e.TagFormat,
				Source:    e.Source,
				Branch:    e.Branch,
			}
		}
	}

	if a.ChangelogGenerator != "" {
		cfg.Changelog = &config.ContentDriver{
			Generator: a.ChangelogGenerator,
			Output:    a.ChangelogOutput,
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
		for _, p := range a.Platforms {
			plat := config.Platform{
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
