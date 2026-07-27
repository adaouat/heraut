// Package forge resolves a heraut config into one or more port.ForgeIdentity values, filling
// gaps from the ambient CI environment or the git origin remote when the config leaves fields
// unset. See ADR-0043 and docs/superpowers/specs/2026-07-24-forge-abstraction-design.md §3.
package forge

import (
	"errors"
	"fmt"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
)

// ErrAmbiguousForge is returned by Resolve when zero-config auto-detection finds more than one
// candidate forge type and nothing (CI, git origin) pins a single one.
var ErrAmbiguousForge = errors.New("ambiguous forge")

// candidateTypes is the fixed set of forge types considered during zero-config auto-detection,
// in the order they appear in the design spec's CI table (§3).
var candidateTypes = []string{"gitlab", "github", "azure_devops"}

// Resolved is the outcome of Resolve: the resolved forge identities (by config order, or the
// single auto-detected one) and the index into Forges that is the enrichment source.
type Resolved struct {
	Forges          []port.ForgeIdentity
	EnrichmentIndex int
}

// Resolve builds one port.ForgeIdentity per cfg.Forges entry (filling gaps from CI/git origin),
// or — when cfg.Forges is empty — auto-detects exactly one forge from the CI environment or the
// git origin remote. getenv and gitOrigin are injected for testability; production callers pass
// os.Getenv and the parsed `git remote get-url origin`.
func Resolve(cfg *config.Config, getenv func(string) string, gitOrigin string) (Resolved, error) {
	if len(cfg.Forges) > 0 {
		return resolveExplicit(cfg, getenv, gitOrigin)
	}
	return resolveAuto(getenv, gitOrigin)
}

// resolveExplicit builds one ForgeIdentity per cfg.Forges entry. Each field fills gaps in order:
// explicit config → CI env (when the CI type matches the entry's platform) → git origin (when
// its detected type matches the entry's platform) → a type default (host, token env).
func resolveExplicit(cfg *config.Config, getenv func(string) string, gitOrigin string) (Resolved, error) {
	ciType, ciHost, ciAPIURL, ciProject, ciToken, ciKind, ciOK := detectCIForge(getenv)
	originType, originHost, originProject, originOK := parseGitOrigin(gitOrigin)

	forges := make([]port.ForgeIdentity, len(cfg.Forges))
	for i, f := range cfg.Forges {
		ciMatches := ciOK && ciType == f.Type
		originMatches := originOK && originType == f.Type

		apiMode := f.APIMode
		if apiMode == "" {
			apiMode = "rest"
		}

		token, kind := resolveToken(f.TokenEnv, getenv, ciMatches, ciToken, ciKind, defaultTokenEnvFor(f.Type))

		forges[i] = port.ForgeIdentity{
			Type:       f.Type,
			Host:       resolveField(f.BaseURL, ciMatches, ciHost, originMatches, originHost, defaultHostFor(f.Type)),
			APIURL:     resolveField(f.APIURL, ciMatches, ciAPIURL, false, "", ""),
			Project:    resolveField(configProject(f), ciMatches, ciProject, originMatches, originProject, ""),
			Repository: repositoryFor(f),
			Token:      token,
			TokenKind:  kind,
			APIMode:    apiMode,
		}
	}

	idx := 0
	if cfg.Commits != nil && cfg.Commits.EnrichmentForge != "" {
		for i, f := range cfg.Forges {
			if f.Name == cfg.Commits.EnrichmentForge {
				idx = i
				break
			}
		}
	}

	return Resolved{Forges: forges, EnrichmentIndex: idx}, nil
}

// resolveAuto auto-detects exactly one forge for zero-config repos: the CI environment pins the
// type when present, else the git origin host does when it's a known public host, else the
// ambient tokens are inspected to find a single unambiguous candidate. Zero candidates means
// offline (empty Resolved, not an error) and more than one means ErrAmbiguousForge.
func resolveAuto(getenv func(string) string, gitOrigin string) (Resolved, error) {
	if ciType, ciHost, ciAPIURL, ciProject, ciToken, ciKind, ok := detectCIForge(getenv); ok {
		token, kind := ciToken, ciKind
		if token == "" {
			token, kind = tokenFromDefaultEnv(getenv, defaultTokenEnvFor(ciType))
		}
		host := ciHost
		if host == "" {
			host = defaultHostFor(ciType)
		}
		return single(port.ForgeIdentity{
			Type:      ciType,
			Host:      host,
			APIURL:    ciAPIURL,
			Project:   ciProject,
			Token:     token,
			TokenKind: kind,
			APIMode:   "rest",
		}), nil
	}

	if originType, originHost, originProject, ok := parseGitOrigin(gitOrigin); ok {
		token, kind := tokenFromDefaultEnv(getenv, defaultTokenEnvFor(originType))
		return single(port.ForgeIdentity{
			Type:      originType,
			Host:      originHost,
			Project:   originProject,
			Token:     token,
			TokenKind: kind,
			APIMode:   "rest",
		}), nil
	}

	var candidates []string
	for _, typ := range candidateTypes {
		if getenv(defaultTokenEnvFor(typ)) != "" {
			candidates = append(candidates, typ)
		}
	}

	switch len(candidates) {
	case 0:
		return Resolved{}, nil
	case 1:
		typ := candidates[0]
		token, kind := tokenFromDefaultEnv(getenv, defaultTokenEnvFor(typ))
		return single(port.ForgeIdentity{
			Type:      typ,
			Host:      defaultHostFor(typ),
			Token:     token,
			TokenKind: kind,
			APIMode:   "rest",
		}), nil
	default:
		return Resolved{}, fmt.Errorf("detected candidates %v and no CI/origin to disambiguate: %w", candidates, ErrAmbiguousForge)
	}
}

// single wraps one ForgeIdentity into a Resolved with EnrichmentIndex 0.
func single(id port.ForgeIdentity) Resolved {
	return Resolved{Forges: []port.ForgeIdentity{id}, EnrichmentIndex: 0}
}

// configProject returns the project-path field of a config.Forge relevant to its platform type:
// github stores it under Repository ("owner/repo"), gitlab and azure_devops under Project.
func configProject(f config.Forge) string {
	if f.Type == "github" {
		return f.Repository
	}
	return f.Project
}

// repositoryFor returns the identity's Repository field: only azure_devops separates the
// repository name from the project path (organization/project + repository). GitHub and GitLab
// carry the full path in Project and leave this empty.
func repositoryFor(f config.Forge) string {
	if f.Type == "azure_devops" {
		return f.Repository
	}
	return ""
}

// resolveField fills a single field's gap, in precedence order: explicit → CI (when it applies)
// → git origin (when it applies) → fallback.
func resolveField(explicit string, ciApplies bool, ciValue string, originApplies bool, originValue string, fallback string) string {
	if explicit != "" {
		return explicit
	}
	if ciApplies && ciValue != "" {
		return ciValue
	}
	if originApplies && originValue != "" {
		return originValue
	}
	return fallback
}

// resolveToken fills the token/tokenKind pair, in precedence order: an explicit token_env (kind
// Private, since it names a personal/project token) → the CI token (when it applies) → the
// type's default token env (kind Private).
func resolveToken(tokenEnv string, getenv func(string) string, ciApplies bool, ciToken string, ciKind port.TokenKind, defaultTokenEnv string) (string, port.TokenKind) {
	if tokenEnv != "" {
		return getenv(tokenEnv), port.TokenPrivate
	}
	if ciApplies && ciToken != "" {
		return ciToken, ciKind
	}
	return tokenFromDefaultEnv(getenv, defaultTokenEnv)
}

// tokenFromDefaultEnv reads a type's default token env var, reporting TokenNone when unset.
func tokenFromDefaultEnv(getenv func(string) string, defaultTokenEnv string) (string, port.TokenKind) {
	if defaultTokenEnv == "" {
		return "", port.TokenNone
	}
	if tok := getenv(defaultTokenEnv); tok != "" {
		return tok, port.TokenPrivate
	}
	return "", port.TokenNone
}
