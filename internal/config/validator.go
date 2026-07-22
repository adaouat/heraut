package config

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

var (
	validStrategies = map[string]bool{
		"semver": true, "calver": true,
		"semver-per-env": true, "calver-per-env": true,
	}
	validGenerators = map[string]bool{
		"native": true, "git-cliff": true, "communique": true,
	}
	validPlatforms = map[string]bool{
		"github": true, "gitlab": true,
	}
	validTagTypes = map[string]bool{
		"annotated": true, "lightweight": true,
	}
	validRemoteMetadata = map[string]bool{
		"required": true, "optional": true, "disabled": true,
	}
	commitTypePattern = regexp.MustCompile(`^\w+$`)
)

// Validate runs all semantic validation layers against cfg.
// All errors are collected and returned; validation does not stop on the first error.
func Validate(cfg *Config) ValidationErrors {
	if cfg == nil {
		return nil
	}
	var errs ValidationErrors
	errs = append(errs, validateRequired(cfg)...)
	errs = append(errs, validateEnums(cfg)...)
	errs = append(errs, validateStrategySpecific(cfg)...)
	errs = append(errs, validateEnvContradictions(cfg.Environments)...)
	errs = append(errs, validateTickets(cfg)...)
	errs = append(errs, validateCommits(cfg)...)
	errs = append(errs, validateRendering(cfg)...)
	return errs
}

// validateTickets checks the ticket-link config: each pattern compiles as a regex, each url
// is an absolute http(s) URL containing {ticket}, and tickets require the git-cliff generator.
func validateTickets(cfg *Config) []ValidationError {
	tickets := cfg.Tickets()
	if len(tickets) == 0 {
		return nil
	}
	var errs []ValidationError
	if !ticketsGeneratorSupported(cfg) {
		errs = append(errs, ValidationError{
			Path:    "commits.tickets",
			Message: "ticket linking requires the git-cliff or native generator",
			Hint:    "set changelog.generator / release.notes.generator to git-cliff or native, or remove tickets",
		})
	}
	for i, t := range tickets {
		base := fmt.Sprintf("commits.tickets[%d]", i)
		if t.Pattern == "" {
			errs = append(errs, ValidationError{Path: base + ".pattern", Message: "required", Hint: "a regex matching ticket IDs, e.g. [A-Z]+-[0-9]+"})
		} else if _, err := regexp.Compile(t.Pattern); err != nil {
			errs = append(errs, ValidationError{Path: base + ".pattern", Message: fmt.Sprintf("invalid regex: %v", err), Hint: "use a valid git-cliff (Rust) / Go regex"})
		}
		if t.URL == "" {
			errs = append(errs, ValidationError{Path: base + ".url", Message: "required", Hint: "an http(s) URL containing {ticket}, e.g. https://jira.example.com/browse/{ticket}"})
			continue
		}
		if !strings.Contains(t.URL, "{ticket}") {
			errs = append(errs, ValidationError{Path: base + ".url", Message: "must contain the {ticket} placeholder", Hint: "e.g. https://jira.example.com/browse/{ticket}"})
		}
		if !isValidBaseURL(strings.ReplaceAll(t.URL, "{ticket}", "1")) {
			errs = append(errs, ValidationError{Path: base + ".url", Message: "must be an absolute http(s) URL", Hint: "include the scheme, e.g. https://…"})
		}
	}
	return errs
}

// validateCommits validates the commits block: each types[i] has a valid, unique type name,
// and scopes_restricted requires a non-empty scopes list. An empty types list is valid — it
// means "use the built-in defaults" (EffectiveTypes merges over them).
func validateCommits(cfg *Config) []ValidationError {
	if cfg.Commits == nil {
		return nil
	}
	var errs []ValidationError
	seen := make(map[string]int)
	for i, t := range cfg.Commits.Types {
		path := fmt.Sprintf("commits.types[%d].name", i)
		if t.Name == "" {
			errs = append(errs, ValidationError{Path: path, Message: "required"})
			continue
		}
		if !commitTypePattern.MatchString(t.Name) {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("%q is not a valid type name", t.Name),
				Hint:    "type names must be a single word (letters/digits/underscore), e.g. feat, fix, docs",
			})
			continue
		}
		if first, ok := seen[t.Name]; ok {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("duplicate type %q (already listed at types[%d])", t.Name, first),
			})
			continue
		}
		seen[t.Name] = i
	}
	scopeSeen := make(map[string]int)
	for i, s := range cfg.Commits.Scopes {
		path := fmt.Sprintf("commits.scopes[%d].name", i)
		if s.Name == "" {
			errs = append(errs, ValidationError{Path: path, Message: "required"})
			continue
		}
		if first, ok := scopeSeen[s.Name]; ok {
			errs = append(errs, ValidationError{
				Path:    path,
				Message: fmt.Sprintf("duplicate scope %q (already listed at scopes[%d])", s.Name, first),
			})
			continue
		}
		scopeSeen[s.Name] = i
	}
	if cfg.Commits.ScopesRestricted && len(EffectiveScopes(cfg.Commits.Scopes)) == 0 {
		errs = append(errs, ValidationError{
			Path:    "commits.scopes_restricted",
			Message: "requires a non-empty commits.scopes list",
			Hint:    "list the allowed scopes, or set scopes_restricted: false",
		})
	}
	return errs
}

// validateRendering validates the rendering block: each exclude sets exactly one of type or
// regex, and any regex compiles.
func validateRendering(cfg *Config) []ValidationError {
	if cfg.Rendering == nil {
		return nil
	}
	var errs []ValidationError
	for i, e := range cfg.Rendering.Excludes {
		path := fmt.Sprintf("rendering.excludes[%d]", i)
		switch {
		case e.Type == "" && e.Regex == "":
			errs = append(errs, ValidationError{Path: path, Message: "must set exactly one of type or regex", Hint: `e.g. {type: chore} or {regex: "^chore\\(deps"}`})
		case e.Type != "" && e.Regex != "":
			errs = append(errs, ValidationError{Path: path, Message: "set only one of type or regex, not both"})
		case e.Regex != "":
			if _, err := regexp.Compile(e.Regex); err != nil {
				errs = append(errs, ValidationError{Path: path + ".regex", Message: fmt.Sprintf("invalid regex: %v", err)})
			}
		}
	}
	if len(cfg.Rendering.Templates) > 0 && !allContentGeneratorsNative(cfg) {
		errs = append(errs, ValidationError{
			Path:    "rendering.templates",
			Message: "rendering.templates requires the native generator",
			Hint:    "set changelog.generator / release.notes.generator to native, or remove rendering.templates",
		})
	}
	errs = append(errs, validateTemplateSnippets(cfg.Rendering.Templates, "rendering.templates")...)
	return errs
}

// allContentGeneratorsNative reports whether every configured content generator is native
// (the only generator whose output is driven by rendering.templates / the template file).
// An empty generator (inherits the default) is allowed.
func allContentGeneratorsNative(cfg *Config) bool {
	drivers := []*ContentDriver{cfg.Changelog}
	if cfg.Release != nil {
		drivers = append(drivers, cfg.Release.Notes)
	}
	for _, d := range drivers {
		if d != nil && d.Generator != "" && !strings.EqualFold(d.Generator, "native") {
			return false
		}
	}
	return true
}

// validateTemplateSnippets parses each inline template snippet under pathPrefix, reporting a
// clear error keyed by block name when one fails to parse (ADR-0037).
func validateTemplateSnippets(snippets map[string]string, pathPrefix string) []ValidationError {
	var errs []ValidationError
	keys := make([]string, 0, len(snippets))
	for k := range snippets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := parseTemplateSnippet(snippets[k]); err != nil {
			errs = append(errs, ValidationError{
				Path:    pathPrefix + "." + k,
				Message: fmt.Sprintf("invalid template: %v", err),
				Hint:    "must be a valid Go text/template snippet",
			})
		}
	}
	return errs
}

// parseTemplateSnippet reports whether s parses as a Go text/template with the native funcs
// available. The funcs are stubs — validation only checks that the snippet parses. The names
// must mirror internal/generators/native.templateFuncs (config cannot import native).
func parseTemplateSnippet(s string) error {
	_, err := template.New("snippet").Funcs(templateFuncStubs()).Parse(s)
	return err
}

func templateFuncStubs() template.FuncMap {
	stub := func(...any) any { return nil }
	return template.FuncMap{
		"upperFirst": stub, "date": stub, "join": stub, "list": stub, "indent": stub, "trim": stub,
	}
}

// ticketsGeneratorSupported reports whether every configured top-level content generator is
// git-cliff (the only generator with a link mechanism). An empty generator (inherits the
// default) is allowed.
func ticketsGeneratorSupported(cfg *Config) bool {
	drivers := []*ContentDriver{cfg.Changelog}
	if cfg.Release != nil {
		drivers = append(drivers, cfg.Release.Notes)
	}
	for _, d := range drivers {
		if d != nil && d.Generator != "" && !strings.EqualFold(d.Generator, "git-cliff") && !strings.EqualFold(d.Generator, "native") {
			return false
		}
	}
	return true
}

func validateRequired(cfg *Config) []ValidationError {
	var errs []ValidationError
	if cfg.Version == "" {
		errs = append(errs, ValidationError{
			Path:    "version",
			Message: "required",
			Hint:    `set version: "1"`,
		})
	} else if cfg.Version != "1" {
		errs = append(errs, ValidationError{
			Path:    "version",
			Message: fmt.Sprintf("%q is not a valid schema version", cfg.Version),
			Hint:    `the only valid schema version is "1"`,
		})
	}
	if cfg.Versioning.Strategy == "" {
		errs = append(errs, ValidationError{
			Path:    "versioning.strategy",
			Message: "required",
			Hint:    "set strategy to one of: semver, calver, semver-per-env, calver-per-env",
		})
	}
	return errs
}

func validateEnums(cfg *Config) []ValidationError {
	var errs []ValidationError
	if cfg.Versioning.Strategy != "" && !validStrategies[cfg.Versioning.Strategy] {
		errs = append(errs, ValidationError{
			Path:    "versioning.strategy",
			Message: fmt.Sprintf("%q is not a valid strategy", cfg.Versioning.Strategy),
			Hint:    "valid strategies: semver, calver, semver-per-env, calver-per-env",
		})
	}
	if cfg.Versioning.TagType != "" && !validTagTypes[cfg.Versioning.TagType] {
		errs = append(errs, ValidationError{
			Path:    "versioning.tag_type",
			Message: fmt.Sprintf("%q is not a valid tag type", cfg.Versioning.TagType),
			Hint:    "valid tag types: annotated, lightweight",
		})
	}
	if rm := cfg.RemoteMetadata(); rm != "" && !validRemoteMetadata[rm] {
		errs = append(errs, ValidationError{
			Path:    "commits.remote_metadata",
			Message: fmt.Sprintf("%q is not a valid remote_metadata policy", rm),
			Hint:    "valid values: required, optional, disabled",
		})
	}
	errs = append(errs, validateContentDriver(cfg.Changelog, "changelog")...)
	errs = append(errs, validateRelease(cfg.Release, "release")...)
	for envName, env := range cfg.Environments {
		base := "environments." + envName
		// Per-env content drivers merge over the top-level (ADR-0019); validate the
		// effective merged driver so an inherited generator satisfies the required check.
		if env.Changelog != nil {
			eff := MergeContentDriver(cfg.Changelog, env.Changelog)
			errs = append(errs, validateContentDriver(eff, base+".changelog")...)
		}
		errs = append(errs, validateEnvRelease(env.Release, cfg.Release, base+".release")...)
	}
	return errs
}

func validateContentDriver(d *ContentDriver, path string) []ValidationError {
	if d == nil {
		return nil
	}
	var errs []ValidationError
	if d.Generator == "" {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: "required",
			Hint:    "set generator to one of: native, git-cliff, communique",
		})
	} else if !validGenerators[d.Generator] {
		errs = append(errs, ValidationError{
			Path:    path + ".generator",
			Message: fmt.Sprintf("%q is not a valid generator", d.Generator),
			Hint:    "valid generators: native, git-cliff, communique",
		})
	}
	if d.TagPattern != "" && d.Generator != "" &&
		!strings.EqualFold(d.Generator, "git-cliff") && !strings.EqualFold(d.Generator, "native") {
		errs = append(errs, ValidationError{
			Path:    path + ".tag_pattern",
			Message: "tag_pattern requires the git-cliff or native generator",
			Hint:    fmt.Sprintf("set generator to git-cliff or native, or remove tag_pattern (current generator: %s)", d.Generator),
		})
	}
	// With native, tag_pattern is a Go regex applied in-process; validate it compiles.
	if d.TagPattern != "" && strings.EqualFold(d.Generator, "native") {
		if _, err := regexp.Compile(d.TagPattern); err != nil {
			errs = append(errs, ValidationError{
				Path:    path + ".tag_pattern",
				Message: fmt.Sprintf("invalid regex: %v", err),
				Hint:    "with generator: native, tag_pattern is a Go regex (e.g. ^v.*-prod$)",
			})
		}
	}
	errs = append(errs, validateContentDriverTemplates(d, path)...)
	errs = append(errs, validateContentDriverRemote(d, path)...)
	return errs
}

// validateContentDriverTemplates validates a driver's native template customization (ADR-0037):
// rendering.templates and the template file require generator: native; each inline snippet parses;
// the template file, when set, exists and parses.
func validateContentDriverTemplates(d *ContentDriver, path string) []ValidationError {
	isNative := strings.EqualFold(d.Generator, "native")
	hasInline := d.Rendering != nil && len(d.Rendering.Templates) > 0
	var errs []ValidationError

	if d.Template != "" && d.Generator != "" && !isNative {
		errs = append(errs, ValidationError{
			Path:    path + ".template",
			Message: "template requires the native generator",
			Hint:    fmt.Sprintf("set generator to native, or remove template (current generator: %s)", d.Generator),
		})
	}
	if hasInline && d.Generator != "" && !isNative {
		errs = append(errs, ValidationError{
			Path:    path + ".rendering.templates",
			Message: "rendering.templates requires the native generator",
			Hint:    fmt.Sprintf("set generator to native, or remove rendering.templates (current generator: %s)", d.Generator),
		})
	}

	if hasInline {
		errs = append(errs, validateTemplateSnippets(d.Rendering.Templates, path+".rendering.templates")...)
	}
	if d.Template != "" && (isNative || d.Generator == "") {
		if b, err := os.ReadFile(d.Template); err != nil {
			errs = append(errs, ValidationError{
				Path:    path + ".template",
				Message: fmt.Sprintf("template file not readable: %v", err),
				Hint:    "the path is relative to the project root; check it exists and is readable",
			})
		} else if perr := parseTemplateSnippet(string(b)); perr != nil {
			errs = append(errs, ValidationError{
				Path:    path + ".template",
				Message: fmt.Sprintf("invalid template: %v", perr),
				Hint:    "the file must be a valid Go text/template",
			})
		}
	}
	return errs
}

// validateContentDriverRemote validates an explicit changelog.remote block (ADR-0026): it
// is only valid on the changelog content driver (release notes already resolve PR/MR
// metadata from release.platforms), requires the git-cliff generator, and requires a valid
// type with that type's required fields.
func validateContentDriverRemote(d *ContentDriver, path string) []ValidationError {
	if d.Remote == nil {
		return nil
	}
	remotePath := path + ".remote"
	if strings.HasSuffix(path, ".notes") {
		return []ValidationError{{
			Path:    remotePath,
			Message: "remote is only valid on the changelog content driver, not release notes",
			Hint:    "release notes already resolve PR/MR metadata from release.platforms; remove this block",
		}}
	}
	var errs []ValidationError
	if d.Generator != "" && d.Generator != "git-cliff" {
		errs = append(errs, ValidationError{
			Path:    remotePath,
			Message: "remote requires the git-cliff generator",
			Hint:    fmt.Sprintf("set generator to git-cliff, or remove remote (current generator: %s)", d.Generator),
		})
	}
	r := d.Remote
	switch r.Type {
	case "":
		errs = append(errs, ValidationError{
			Path:    remotePath + ".type",
			Message: "required",
			Hint:    "set type to one of: github, gitlab, azure_devops",
		})
	case "github":
		if r.Repository == "" {
			errs = append(errs, ValidationError{
				Path:    remotePath + ".repository",
				Message: "required for type: github",
				Hint:    `set repository to "owner/repo"`,
			})
		}
	case "gitlab":
		if r.Project == "" {
			errs = append(errs, ValidationError{
				Path:    remotePath + ".project",
				Message: "required for type: gitlab",
				Hint:    `set project to "namespace/repo" (or "namespace/subgroup/repo")`,
			})
		}
	case "azure_devops":
		if r.Project == "" {
			errs = append(errs, ValidationError{
				Path:    remotePath + ".project",
				Message: "required for type: azure_devops",
				Hint:    `set project to "organization/project"`,
			})
		}
		if r.Repository == "" {
			errs = append(errs, ValidationError{
				Path:    remotePath + ".repository",
				Message: "required for type: azure_devops",
				Hint:    "set repository to your Azure DevOps repository name",
			})
		}
	default:
		errs = append(errs, ValidationError{
			Path:    remotePath + ".type",
			Message: fmt.Sprintf("%q is not a valid remote type", r.Type),
			Hint:    "valid types: github, gitlab, azure_devops",
		})
	}
	if r.BaseURL != "" && !isValidBaseURL(strings.TrimRight(r.BaseURL, "/")) {
		errs = append(errs, ValidationError{
			Path:    remotePath + ".base_url",
			Message: fmt.Sprintf("%q is not a valid URL", r.BaseURL),
			Hint:    "base_url must be an absolute http(s) URL, e.g. https://git.example.com",
		})
	}
	return errs
}

func validateRelease(r *Release, path string) []ValidationError {
	if r == nil {
		return nil
	}
	var errs []ValidationError
	errs = append(errs, validateContentDriver(r.Notes, path+".notes")...)
	errs = append(errs, validatePlatformEntries(r.Platforms, path)...)
	return errs
}

// validatePlatformEntries validates one release.platforms list (top-level or a single
// environment override): each entry's platform type, its required unique name
// (scoped to this list — ADR-0025), and its base_url.
func validatePlatformEntries(platforms []Platform, path string) []ValidationError {
	var errs []ValidationError
	seen := make(map[string]int)
	for i, plat := range platforms {
		platPath := fmt.Sprintf("%s.platforms[%d]", path, i)
		if plat.Type == "" {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: "required",
				Hint:    "set platform to one of: github, gitlab",
			})
		} else if !validPlatforms[plat.Type] {
			errs = append(errs, ValidationError{
				Path:    platPath + ".platform",
				Message: fmt.Sprintf("%q is not a valid platform", plat.Type),
				Hint:    "valid platforms: github, gitlab",
			})
		}
		if plat.Name == "" {
			errs = append(errs, ValidationError{
				Path:    platPath + ".name",
				Message: "required",
				Hint:    `set a unique name for this platform entry, e.g. "gitlab-saas"`,
			})
		} else if first, ok := seen[plat.Name]; ok {
			errs = append(errs, ValidationError{
				Path:    platPath + ".name",
				Message: fmt.Sprintf("duplicate platform name %q (already used by platforms[%d])", plat.Name, first),
				Hint:    "platform names must be unique within this release.platforms list",
			})
		} else {
			seen[plat.Name] = i
		}
		errs = append(errs, validatePlatformBaseURL(plat, platPath)...)
	}
	return errs
}

// validatePlatformBaseURL validates a platform's base_url: it must be a well-formed
// absolute http(s) URL. An empty value means "use the platform-type default" and is
// always accepted. Self-hosted (non-default) hosts are accepted — see ADR-0025, which
// supersedes ADR-0020's gate.
func validatePlatformBaseURL(plat Platform, platPath string) []ValidationError {
	if plat.BaseURL == "" {
		return nil
	}
	raw := strings.TrimRight(plat.BaseURL, "/")
	if !isValidBaseURL(raw) {
		return []ValidationError{{
			Path:    platPath + ".base_url",
			Message: fmt.Sprintf("%q is not a valid URL", plat.BaseURL),
			Hint:    "base_url must be an absolute http(s) URL, e.g. https://gitlab.example.com",
		}}
	}
	return nil
}

func isValidBaseURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

func validateEnvRelease(r *EnvRelease, topRelease *Release, path string) []ValidationError {
	if r == nil {
		return nil
	}
	var errs []ValidationError
	// Per-env notes merge over the top-level release.notes (ADR-0019); validate the
	// effective merged driver.
	if r.Notes != nil {
		var topNotes *ContentDriver
		if topRelease != nil {
			topNotes = topRelease.Notes
		}
		errs = append(errs, validateContentDriver(MergeContentDriver(topNotes, r.Notes), path+".notes")...)
	}
	errs = append(errs, validatePlatformEntries(r.Platforms, path)...)
	return errs
}

func validateStrategySpecific(cfg *Config) []ValidationError {
	var errs []ValidationError

	// Flat-strategy guard: environments is only valid with per-env strategies.
	if len(cfg.Environments) > 0 {
		switch cfg.Versioning.Strategy {
		case "semver-per-env", "calver-per-env":
			// expected
		default:
			if cfg.Versioning.Strategy != "" {
				errs = append(errs, ValidationError{
					Path:    "environments",
					Message: fmt.Sprintf("environments is only valid with semver-per-env or calver-per-env (current strategy: %s)", cfg.Versioning.Strategy),
					Hint:    "remove the environments block, or change the strategy to semver-per-env or calver-per-env",
				})
			}
		}
	}

	switch cfg.Versioning.Strategy {
	case "calver", "calver-per-env":
		if cfg.Versioning.Format == "" {
			errs = append(errs, ValidationError{
				Path:    "versioning.format",
				Message: fmt.Sprintf("required for %s strategy", cfg.Versioning.Strategy),
				Hint:    `set a CalVer format string, e.g. "YYYY.MM.PATCH"`,
			})
		}
	}
	switch cfg.Versioning.Strategy {
	case "semver-per-env", "calver-per-env":
		errs = append(errs, validatePerEnv(cfg)...)
	}
	return errs
}

func validatePerEnv(cfg *Config) []ValidationError {
	var errs []ValidationError
	envs := cfg.Environments

	if len(envs) == 0 {
		return append(errs, ValidationError{
			Path:    "environments",
			Message: fmt.Sprintf("required for %s strategy", cfg.Versioning.Strategy),
			Hint:    "define at least one environment with its tag_format and bump mode",
		})
	}

	// Common tag_format must contain {version} if set.
	if cfg.Versioning.TagFormat != "" && !strings.Contains(cfg.Versioning.TagFormat, "{version}") {
		errs = append(errs, ValidationError{
			Path:    "versioning.tag_format",
			Message: "must contain {version}",
			Hint:    `example: "{env}/{version}"`,
		})
	}

	// Count auto envs for ambiguity detection.
	autoCount := 0
	for _, env := range envs {
		if env.Bump == "auto" {
			autoCount++
		}
	}

	for _, envName := range sortedEnvKeys(envs) {
		env := envs[envName]
		envPath := "environments." + envName

		// bump required and enum.
		if env.Bump == "" {
			errs = append(errs, ValidationError{
				Path:    envPath + ".bump",
				Message: "required",
				Hint:    "set bump to auto or promote",
			})
		} else if env.Bump != "auto" && env.Bump != "promote" {
			errs = append(errs, ValidationError{
				Path:    envPath + ".bump",
				Message: fmt.Sprintf("%q is not a valid bump mode for per-env strategy", env.Bump),
				Hint:    "valid modes: auto, promote",
			})
		}

		// tag_format must contain {version} if set.
		if env.TagFormat != "" && !strings.Contains(env.TagFormat, "{version}") {
			errs = append(errs, ValidationError{
				Path:    envPath + ".tag_format",
				Message: "must contain {version}",
				Hint:    fmt.Sprintf(`example: "%s/{version}"`, envName),
			})
		}

		// Every env must have a tag_format, either directly or via the common one.
		if env.TagFormat == "" && cfg.Versioning.TagFormat == "" {
			errs = append(errs, ValidationError{
				Path:    envPath + ".tag_format",
				Message: "required: no tag_format set on this environment or at versioning.tag_format",
				Hint:    "set tag_format on this environment or set versioning.tag_format as a shared template using {env} and {version}",
			})
		}

		// source validation (ADR-0008).
		sourcePath := envPath + ".source"
		if env.Bump == "auto" && env.Source != "" {
			errs = append(errs, ValidationError{
				Path:    sourcePath,
				Message: "source is not valid on bump: auto environments",
				Hint:    "remove source: from this environment; source is only meaningful for bump: promote",
			})
		}
		if env.Bump == "promote" {
			if env.Source != "" {
				if _, exists := envs[env.Source]; !exists {
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: fmt.Sprintf("environment %q does not exist", env.Source),
						Hint:    "available environments: " + strings.Join(sortedEnvKeys(envs), ", "),
					})
				}
				if env.Source == envName {
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: "environment cannot promote from itself",
						Hint:    "set source to a different environment",
					})
				}
			} else {
				switch {
				case autoCount == 0:
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: "no auto environment found to promote from",
						Hint:    "add source: <env-name> or add an environment with bump: auto",
					})
				case autoCount > 1:
					errs = append(errs, ValidationError{
						Path:    sourcePath,
						Message: "multiple auto environments exist; source is ambiguous",
						Hint:    "add source: <env-name> to specify which environment to promote from",
					})
				}
			}
		}
	}

	errs = append(errs, detectCycles(envs)...)
	return errs
}

func validateEnvContradictions(envs map[string]Environment) []ValidationError {
	var errs []ValidationError
	for _, envName := range sortedEnvKeys(envs) {
		env := envs[envName]
		base := "environments." + envName

		if env.DisableChangelog && env.Changelog != nil {
			errs = append(errs, ValidationError{
				Path:    base + ".changelog",
				Message: "disable_changelog: true makes the changelog override unreachable",
				Hint:    "remove either disable_changelog: true (to apply the override) or the changelog: block (to keep the step disabled)",
			})
		}
		if env.DisableNotes && env.Release != nil && env.Release.Notes != nil {
			errs = append(errs, ValidationError{
				Path:    base + ".release.notes",
				Message: "disable_notes: true makes the release notes override unreachable",
				Hint:    "remove either disable_notes: true (to apply the override) or the release.notes: block (to keep the step disabled)",
			})
		}
	}
	return errs
}

func detectCycles(envs map[string]Environment) []ValidationError {
	var errs []ValidationError
	reported := map[string]bool{}

	for _, envName := range sortedEnvKeys(envs) {
		if reported[envName] {
			continue
		}
		if envs[envName].Bump != "promote" {
			continue
		}
		if found, path := findCycle(envs, envName); found {
			errs = append(errs, ValidationError{
				Path:    "environments." + envName + ".source",
				Message: "cycle detected (" + strings.Join(path, " → ") + ")",
				Hint:    "each promotion source must trace back to an auto env without revisiting envs",
			})
			for _, e := range path {
				reported[e] = true
			}
		}
	}
	return errs
}

func findCycle(envs map[string]Environment, start string) (bool, []string) {
	path := []string{start}
	seen := map[string]bool{start: true}
	current := start

	for {
		env, exists := envs[current]
		if !exists {
			return false, nil
		}
		src := env.Source
		if src == "" {
			return false, nil
		}
		if seen[src] {
			return true, append(path, src)
		}
		seen[src] = true
		path = append(path, src)
		current = src
	}
}

func sortedEnvKeys(envs map[string]Environment) []string {
	keys := make([]string, 0, len(envs))
	for k := range envs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
