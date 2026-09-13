package app

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/conventionalcommit"
	"github.com/adaouat/heraut/internal/generators/native"
)

// DefaultCommitTypes is the type allow-list applied when commits.types adds no overrides:
// the names of the built-in default type set (config.EffectiveTypes(nil)).
var DefaultCommitTypes = commitTypeNames(config.EffectiveTypes(nil))

func commitTypeNames(types []config.TypeRule) []string {
	names := make([]string, len(types))
	for i, t := range types {
		names[i] = t.Name
	}
	return names
}

// AllowedCommitTypes returns the effective commit-type allow-list: the names of
// config.EffectiveTypes(commits.types) — the built-in defaults with the user's commits.types
// merged over them. Single source of truth shared by VerifyCommit and the commit wizard.
func AllowedCommitTypes(cfg *config.Config) []string {
	var user []config.TypeRule
	if cfg != nil && cfg.Commits != nil {
		user = cfg.Commits.Types
	}
	return commitTypeNames(config.EffectiveTypes(user))
}

// CommitSummary is a verified commit's structural breakdown plus any detected
// commits.tickets references, for `heraut commit verify`'s recap output (T242).
type CommitSummary struct {
	Type        string
	Scope       string
	Breaking    bool
	Description string
	Tickets     []native.TicketMatch
}

// VerifyCommit validates message against the conventional-commit grammar and the
// configured (or default) type allow-list. Merge and fixup commits are always skipped,
// unconditionally — nil, nil in that case, since there is no commit to summarize. cfg may
// be nil (no .heraut.yml present) — the default type list applies and no tickets are
// configured to match against.
func VerifyCommit(cfg *config.Config, message string) (*CommitSummary, error) {
	if conventionalcommit.IsMergeCommit(message) || conventionalcommit.IsFixupCommit(message) {
		return nil, nil
	}

	c, err := conventionalcommit.Parse(message)
	if err != nil {
		return nil, fmt.Errorf("validating commit message: %w", err)
	}

	types := AllowedCommitTypes(cfg)
	if !slices.Contains(types, c.Type) {
		return nil, fmt.Errorf("commit type %q is not allowed (allowed: %s)", c.Type, strings.Join(types, ", "))
	}
	if err := verifyScope(cfg, c.Scope); err != nil {
		return nil, err
	}
	if err := verifyRules(cfg, c, message); err != nil {
		return nil, err
	}

	var tickets []config.Ticket
	if cfg != nil {
		tickets = cfg.Tickets()
	}
	return &CommitSummary{
		Type:        c.Type,
		Scope:       c.Scope,
		Breaking:    c.Breaking,
		Description: c.Description,
		Tickets:     native.MatchTickets(message, tickets),
	}, nil
}

// verifyScope rejects a scope outside commits.scopes when scopes_restricted is true. A commit
// with no scope is always allowed — the restriction applies only to scopes that are present.
func verifyScope(cfg *config.Config, scope string) error {
	if cfg == nil || cfg.Commits == nil || !cfg.Commits.ScopesRestricted {
		return nil
	}
	if scope == "" {
		return nil
	}
	names := config.ScopeNames(config.EffectiveScopes(cfg.Commits.Scopes))
	if slices.Contains(names, scope) {
		return nil
	}
	return fmt.Errorf("commit scope %q is not allowed (allowed: %s)", scope, strings.Join(names, ", "))
}

// verifyRules evaluates commits.rules (ADR-0056) against the parsed commit, collecting
// every violation into one error rather than stopping at the first — a single commit can
// trip more than one rule, and `commit check` scans many commits per run.
func verifyRules(cfg *config.Config, c *conventionalcommit.Commit, message string) error {
	if cfg == nil || cfg.Commits == nil || len(cfg.Commits.Rules) == 0 {
		return nil
	}
	tickets := cfg.Tickets()
	var violations []string
	for _, r := range cfg.Commits.Rules {
		if !ruleAppliesTo(r, c) {
			continue
		}
		violated, msg, err := evaluateRule(r, c, message, tickets)
		if err != nil {
			return fmt.Errorf("commits.rules %q: %w", r.Name, err)
		}
		if violated {
			violations = append(violations, msg)
		}
	}
	if len(violations) == 0 {
		return nil
	}
	return fmt.Errorf("commit message violates rule(s): %s", strings.Join(violations, "; "))
}

// ruleAppliesTo reports whether r's optional Types/Scopes scoping matches c. Omitting both
// applies the rule to every commit.
func ruleAppliesTo(r config.CommitRule, c *conventionalcommit.Commit) bool {
	if len(r.Types) > 0 && !slices.Contains(r.Types, c.Type) {
		return false
	}
	if len(r.Scopes) > 0 && !slices.Contains(r.Scopes, c.Scope) {
		return false
	}
	return true
}

// evaluateRule evaluates one rule against the text r.Target selects, returning whether it
// was violated and the message to report. A require_ticket rule with no Message gets a
// synthesized one naming the rule. Regex compile errors are reported rather than silently
// skipped — config.Validate rejects them before this point in the normal CLI flow, but
// VerifyCommit is exported and may be called directly (e.g. in tests) without validation.
func evaluateRule(r config.CommitRule, c *conventionalcommit.Commit, message string, tickets []config.Ticket) (violated bool, msg string, err error) {
	target := ruleTarget(c, message, r.Target)
	switch {
	case r.Deny != "":
		re, cerr := regexp.Compile(r.Deny)
		if cerr != nil {
			return false, "", fmt.Errorf("invalid deny regex: %w", cerr)
		}
		return re.MatchString(target), r.Message, nil
	case r.Require != "":
		re, cerr := regexp.Compile(r.Require)
		if cerr != nil {
			return false, "", fmt.Errorf("invalid require regex: %w", cerr)
		}
		return !re.MatchString(target), r.Message, nil
	case r.RequireTicket:
		if len(native.MatchTickets(target, tickets)) > 0 {
			return false, "", nil
		}
		msg := r.Message
		if msg == "" {
			msg = fmt.Sprintf("commit must reference a ticket matching one of the configured commits.tickets patterns (rule %q)", r.Name)
		}
		return true, msg, nil
	}
	return false, "", nil
}

// ruleTarget extracts the text a rule matches against: the raw header line, the parsed
// body, the joined footer trailers, or (the default, target == "") the full raw message.
func ruleTarget(c *conventionalcommit.Commit, message, target string) string {
	switch target {
	case "header":
		if i := strings.IndexByte(message, '\n'); i >= 0 {
			return message[:i]
		}
		return message
	case "body":
		return c.Body
	case "footer":
		var b strings.Builder
		for _, f := range c.Footers {
			b.WriteString(f.Token)
			b.WriteString(": ")
			b.WriteString(f.Value)
			b.WriteString("\n")
		}
		return b.String()
	default:
		return message
	}
}
