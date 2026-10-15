package config

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	validCommitRuleTargets = map[string]bool{
		"": true, "header": true, "body": true, "footer": true, "message": true,
	}
	commitTypePattern = regexp.MustCompile(`^\w+$`)
)

// validateTickets checks the ticket-link config: each pattern compiles as a regex, and each url
// is an absolute http(s) URL containing {ticket}.
func validateTickets(cfg *Config) []ValidationError {
	tickets := cfg.Tickets()
	if len(tickets) == 0 {
		return nil
	}
	var errs []ValidationError
	for i, t := range tickets {
		base := fmt.Sprintf("commits.tickets[%d]", i)
		if t.Pattern == "" {
			errs = append(errs, ValidationError{Path: base + ".pattern", Message: "required", Hint: "a regex matching ticket IDs, e.g. [A-Z]+-[0-9]+"})
		} else if _, err := regexp.Compile(t.Pattern); err != nil {
			errs = append(errs, ValidationError{Path: base + ".pattern", Message: fmt.Sprintf("invalid regex: %v", err), Hint: "use a valid Go regex"})
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
// scopes_restricted requires a non-empty scopes list, and each rules[i] (ADR-0056) is a
// well-formed CommitRule. An empty types list is valid — it means "use the built-in
// defaults" (EffectiveTypes merges over them).
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
	errs = append(errs, validateCommitRules(cfg.Commits)...)
	return errs
}

// validateCommitRules validates commits.rules (ADR-0056): each rule has a non-empty, unique
// name; sets exactly one of deny/require/require_ticket; deny/require compile as regex and
// require a message; require_ticket requires a non-empty commits.tickets; and target, when
// set, is one of header/body/footer/message.
func validateCommitRules(c *Commits) []ValidationError {
	var errs []ValidationError
	seen := make(map[string]int)
	for i, r := range c.Rules {
		path := fmt.Sprintf("commits.rules[%d]", i)

		if r.Name == "" {
			errs = append(errs, ValidationError{Path: path + ".name", Message: "required"})
		} else if first, ok := seen[r.Name]; ok {
			errs = append(errs, ValidationError{
				Path:    path + ".name",
				Message: fmt.Sprintf("duplicate rule name %q (already listed at rules[%d])", r.Name, first),
			})
		} else {
			seen[r.Name] = i
		}

		set := 0
		for _, isSet := range []bool{r.Deny != "", r.Require != "", r.RequireTicket} {
			if isSet {
				set++
			}
		}
		switch {
		case set == 0:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "must set exactly one of deny, require, or require_ticket",
				Hint:    `e.g. {deny: "(?i)\\bwip\\b"} or {require_ticket: true}`,
			})
		case set > 1:
			errs = append(errs, ValidationError{
				Path:    path,
				Message: "set only one of deny, require, or require_ticket, not more than one",
			})
		}

		if r.Deny != "" {
			if _, err := regexp.Compile(r.Deny); err != nil {
				errs = append(errs, ValidationError{Path: path + ".deny", Message: fmt.Sprintf("invalid regex: %v", err)})
			}
		}
		if r.Require != "" {
			if _, err := regexp.Compile(r.Require); err != nil {
				errs = append(errs, ValidationError{Path: path + ".require", Message: fmt.Sprintf("invalid regex: %v", err)})
			}
		}
		if r.RequireTicket && len(c.Tickets) == 0 {
			errs = append(errs, ValidationError{
				Path:    path + ".require_ticket",
				Message: "requires a non-empty commits.tickets list",
				Hint:    "configure commits.tickets, or remove require_ticket from this rule",
			})
		}
		if (r.Deny != "" || r.Require != "") && r.Message == "" {
			errs = append(errs, ValidationError{
				Path:    path + ".message",
				Message: "required for deny/require rules",
				Hint:    "set a message shown when the rule is violated",
			})
		}
		if !validCommitRuleTargets[r.Target] {
			errs = append(errs, ValidationError{
				Path:    path + ".target",
				Message: fmt.Sprintf("%q is not a valid target", r.Target),
				Hint:    "valid targets: header, body, footer, message",
			})
		}
	}
	return errs
}
