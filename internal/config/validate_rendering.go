package config

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/template"
)

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
	errs = append(errs, validateTemplateSnippets(cfg.Rendering.Templates, "rendering.templates")...)
	var trailers []FooterRule
	if cfg.Rendering.Commit != nil {
		trailers = cfg.Rendering.Commit.Trailers
	}
	errs = append(errs, validateTrailers(trailers)...)
	return errs
}

// validateTrailers validates rendering.templates.commit.trailers (ADR-0057; relocated by
// ADR-0060): each
// rule has a non-empty token, unique case-insensitively across the list, and sets exactly one of
// renderer/hide; renderer, when set, must parse as a valid Go template.
func validateTrailers(rules []FooterRule) []ValidationError {
	var errs []ValidationError
	seen := make(map[string]int)
	for i, r := range rules {
		path := fmt.Sprintf("rendering.templates.commit.trailers[%d]", i)

		if r.Token == "" {
			errs = append(errs, ValidationError{Path: path + ".token", Message: "required"})
		} else {
			key := strings.ToLower(r.Token)
			if first, ok := seen[key]; ok {
				errs = append(errs, ValidationError{
					Path:    path + ".token",
					Message: fmt.Sprintf("duplicate token %q (case-insensitive; already listed at commit.trailers[%d])", r.Token, first),
				})
			} else {
				seen[key] = i
			}
		}

		switch {
		case r.Renderer == "" && !r.Hide:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "must set exactly one of renderer or hide",
				Hint:    `e.g. {renderer: "{{ .Value }}"} or {hide: true}`,
			})
		case r.Renderer != "" && r.Hide:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "set only one of renderer or hide, not both",
			})
		}

		if r.Renderer != "" {
			if err := parseTemplateSnippet(r.Renderer); err != nil {
				errs = append(errs, ValidationError{
					Path:    path + ".renderer",
					Message: fmt.Sprintf("invalid template: %v", err),
					Hint:    "must be a valid Go text/template snippet",
				})
			}
		}
	}
	return errs
}

// validateTemplateSnippets parses each inline template snippet under pathPrefix, reporting a
// clear error keyed by block name when one fails to parse (ADR-0037).
// validTemplateBlocks is the set of native template blocks that rendering.templates may override,
// keyed by the dotted name TemplateOverrides.UnmarshalYAML flattens the release:/commit: YAML
// namespaces into (ADR-0059). It must stay in sync with the {{define "…"}} blocks embedded in
// internal/generators/native (blocks.tmpl / changelog.tmpl / release_notes.tmpl) and with
// schema.json. config cannot import native (layer rule), so the list is maintained here.
var validTemplateBlocks = map[string]bool{
	"title": true, "subtitle": true, "footer": true,
	"release.section": true, "release.group": true, "release.contributors": true,
	"release.stats": true, "release.footer": true,
	"commit.message": true, "commit.ticket": true, "commit.contributor": true,
	"changelog": true, "release_notes": true,
}

const validTemplateBlocksHint = "valid blocks: changelog, commit.contributor, commit.message, commit.ticket, footer, release.contributors, release.footer, release.group, release.section, release.stats, release_notes, subtitle, title"

func validateTemplateSnippets(snippets map[string]string, pathPrefix string) []ValidationError {
	var errs []ValidationError
	keys := make([]string, 0, len(snippets))
	for k := range snippets {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if !validTemplateBlocks[k] {
			errs = append(errs, ValidationError{
				Path:    pathPrefix + "." + k,
				Message: fmt.Sprintf("unknown template block %q", k),
				Hint:    validTemplateBlocksHint,
			})
			continue
		}
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

func validateContentDriver(d *ContentDriver, path string) []ValidationError {
	if d == nil {
		return nil
	}
	var errs []ValidationError
	// tag_pattern is a Go regex applied in-process by the (only) generator, native; validate it
	// compiles. No generator gate needed — native is the only generator (T177/T180).
	if d.TagPattern != "" {
		if _, err := regexp.Compile(d.TagPattern); err != nil {
			errs = append(errs, ValidationError{
				Path:    path + ".tag_pattern",
				Message: fmt.Sprintf("invalid regex: %v", err),
				Hint:    "tag_pattern is a Go regex (e.g. ^v.*-prod$)",
			})
		}
	}
	errs = append(errs, validateContentDriverTemplates(d, path)...)
	return errs
}

// validateContentDriverTemplates validates a driver's template customization (ADR-0037): each
// inline rendering.templates snippet parses, and the template file, when set, exists and parses.
// No generator gate — native is the only generator (T177/T181).
func validateContentDriverTemplates(d *ContentDriver, path string) []ValidationError {
	hasInline := d.Rendering != nil && len(d.Rendering.Templates) > 0
	var errs []ValidationError

	if hasInline {
		errs = append(errs, validateTemplateSnippets(d.Rendering.Templates, path+".rendering.templates")...)
	}
	if d.Template != "" {
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
