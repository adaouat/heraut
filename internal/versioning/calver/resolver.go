package calver

import (
	"fmt"
	"strings"
	"time"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/port"
	"github.com/adaouat/heraut/internal/versioning"
)

// Resolver resolves the next CalVer version from the current date and git tags.
type Resolver struct {
	runner port.Runner
	cfg    *config.Config
	now    func() time.Time
	tokens []Token // cached after first parse
}

// New constructs a CalVer Resolver. now is injectable for deterministic tests.
func New(runner port.Runner, cfg *config.Config, now func() time.Time) *Resolver {
	return &Resolver{runner: runner, cfg: cfg, now: now}
}

// Resolve returns the next CalVer version result.
func (r *Resolver) Resolve() (versioning.Result, error) {
	tokens, err := r.parsedTokens()
	if err != nil {
		return versioning.Result{}, err
	}

	prefix := r.prefix()
	stdout, _, err := r.runner.Run("git", "tag", "-l", prefix+"*", "--sort=-version:refname")
	if err != nil {
		return versioning.Result{}, fmt.Errorf("listing git tags: %w", err)
	}

	rawTags := splitLines(stdout)

	// Find the first tag whose bare form parses as our CalVer format.
	var currentTag string
	var latestValues *Values
	for _, tag := range rawTags {
		bare := strings.TrimPrefix(tag, prefix)
		v, err := ParseVersion(tokens, bare)
		if err == nil {
			currentTag = tag
			latestValues = &v
			break
		}
	}

	version, err := r.computeNext(tokens, latestValues)
	if err != nil {
		return versioning.Result{}, err
	}

	return versioning.Result{
		Version:    version,
		Tag:        prefix + version,
		CurrentTag: currentTag,
	}, nil
}

// BumpAuto is not used by the CalVer calculator; it satisfies the
// VersionCalculator interface for internal/versioning/perenv.
func (r *Resolver) BumpAuto(_ []string, _ []string) (string, error) {
	return "", fmt.Errorf("BumpAuto is not supported by the CalVer calculator")
}

// BumpFromDate computes the next CalVer version from date only, without querying git.
// tags is a caller-provided slice of bare version strings (no prefix), newest first.
// Implements the VersionCalculator interface consumed by internal/versioning/perenv.
func (r *Resolver) BumpFromDate(tags []string) (string, error) {
	tokens, err := r.parsedTokens()
	if err != nil {
		return "", err
	}

	var latestValues *Values
	for _, tag := range tags {
		v, err := ParseVersion(tokens, tag)
		if err == nil {
			latestValues = &v
			break
		}
	}

	return r.computeNext(tokens, latestValues)
}

func (r *Resolver) computeNext(tokens []Token, latest *Values) (string, error) {
	now := r.now()
	nowValues := valuesFromTime(now, r.cfg.Versioning.Sprint, hasWeekToken(tokens))

	var patch int
	if latest == nil {
		patch = 0
	} else {
		if periodKey(tokens, nowValues) == periodKey(tokens, *latest) {
			patch = latest.Patch + 1
		} else {
			patch = 0
		}
	}

	nowValues.Patch = patch
	return RenderVersion(tokens, nowValues), nil
}

func (r *Resolver) parsedTokens() ([]Token, error) {
	if r.tokens != nil {
		return r.tokens, nil
	}
	tokens, err := ParseFormat(r.cfg.Versioning.Format)
	if err != nil {
		return nil, fmt.Errorf("invalid calver format %q: %w", r.cfg.Versioning.Format, err)
	}
	r.tokens = tokens
	return tokens, nil
}

func (r *Resolver) prefix() string {
	if r.cfg.Versioning.TagPrefix != nil {
		return *r.cfg.Versioning.TagPrefix
	}
	return ""
}

// valuesFromTime extracts calendar values from t plus the configured sprint counter. When the
// format carries WW, Year is the ISO year, the only year an ISO week number belongs to: around
// New Year the calendar year would pair a week with the wrong year (2027 with week 53 of 2026) or
// reuse a tag (2025.01.0 on 2025-12-29, which is week 1 of 2026).
func valuesFromTime(t time.Time, sprint int, isoYear bool) Values {
	month := int(t.Month())
	year, isoWeek := t.ISOWeek()
	if !isoYear {
		year = t.Year()
	}
	return Values{
		Year:     year,
		Month:    month,
		Day:      t.Day(),
		Week:     isoWeek,
		Quarter:  (month-1)/3 + 1,
		Semester: (month-1)/6 + 1,
		Sprint:   sprint,
	}
}

func hasWeekToken(tokens []Token) bool {
	for _, tok := range tokens {
		if tok.Kind == KindWW {
			return true
		}
	}
	return false
}

// periodKey returns a string that uniquely identifies the calendar period for
// the given values, based on which non-PATCH tokens appear in the format.
// Two Values with the same period key are in the same period (PATCH increments).
func periodKey(tokens []Token, v Values) string {
	return bucketKey(nonPatchOrder(tokens), v)
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
