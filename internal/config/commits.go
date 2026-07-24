package config

// Commits is heraut's single source of truth for commit semantics and enrichment (ADR-0033):
// the conventional-commit type set, scope rules, ticket links, and the remote-metadata policy.
type Commits struct {
	// TypesHeadingLevel is the heading depth (number of '#') for type sections in rendered
	// output. Zero means the renderer's default.
	TypesHeadingLevel int `yaml:"types_heading_level,omitempty"`
	// Types are the conventional-commit type rules, merged over the built-in defaults by name
	// (see EffectiveTypes). The effective set is the verify allow-list and the section taxonomy.
	Types []TypeRule `yaml:"types,omitempty"`
	// Scopes is the allowed scope list (offered by `heraut commit create`; enforced by
	// `heraut commit verify` only when ScopesRestricted is true). Each entry's description is
	// shown beside the scope in the wizard picker.
	Scopes []ScopeRule `yaml:"scopes,omitempty"`
	// ScopesRestricted, when true, makes `heraut commit verify` reject scopes outside Scopes.
	ScopesRestricted bool `yaml:"scopes_restricted,omitempty"`
	// Tickets configures issue-tracker links matched in commit messages and rendered as links.
	Tickets []Ticket `yaml:"tickets,omitempty"`
	// RemoteMetadata is the PR/MR enrichment policy: "required", "optional" (default), or
	// "disabled". Governs changelog and release-notes generation.
	RemoteMetadata string `yaml:"remote_metadata,omitempty"`
	// EnrichmentForge references a forges[].name used as the PR/MR metadata source for
	// changelog and release-notes generation (ADR-0043). Additive alongside RemoteMetadata.
	EnrichmentForge string `yaml:"enrichment_forge,omitempty"`
	// EnrichmentPolicy is the PR/MR enrichment policy sourced from EnrichmentForge:
	// "required", "optional" (default), or "disabled". Additive alongside RemoteMetadata.
	EnrichmentPolicy string `yaml:"enrichment_policy,omitempty"`
}

// Rendering configures content output (ADR-0033).
type Rendering struct {
	// Excludes drop matched commits from the rendered changelog/release-notes.
	Excludes []Exclude `yaml:"excludes,omitempty"`
	// Templates overrides built-in native template blocks by key (e.g. "commit", "group",
	// "contributor", "header", "footer"): each value is a Go text/template snippet. native
	// only — deep-merged global → per-driver → per-env (ADR-0037).
	Templates map[string]string `yaml:"templates,omitempty"`
}

// Exclude drops matched commits from rendered output. Exactly one of Type or Regex must be
// set: Type matches the conventional-commit type; Regex matches the commit subject.
type Exclude struct {
	Type  string `yaml:"type,omitempty"`
	Regex string `yaml:"regex,omitempty"`
}

// TypeRule configures one conventional-commit type. User entries are merged over the
// built-in defaults by name (see EffectiveTypes): the type word, its changelog section
// label and order, and whether to drop a default type from the effective set.
type TypeRule struct {
	Name        string `yaml:"name"`
	Order       *int   `yaml:"order,omitempty"`       // nil = unordered (sorts after ordered types)
	Render      string `yaml:"render,omitempty"`      // section label; empty = capitalize Name
	Remove      bool   `yaml:"remove,omitempty"`      // drop this default type from the effective set
	Description string `yaml:"description,omitempty"` // one-line hint shown in the commit wizard picker
}

// defaultTypes is the built-in commit-type set merged under user config: the ADR-0027 verify
// types, render-labeled and ordered for changelog sections. revert, the security body rule,
// and the catch-all "Other" group are rendering concerns owned by the native renderer, not
// allow-list types, so they are deliberately absent here (keeping the verify allow-list
// behaviour identical to ADR-0027).
func defaultTypes() []TypeRule {
	order := func(n int) *int { return &n }
	return []TypeRule{
		{Name: "feat", Order: order(0), Render: "🚀 Features", Description: "A new feature"},
		{Name: "fix", Order: order(1), Render: "🐛 Bug Fixes", Description: "A bug fix"},
		{Name: "refactor", Order: order(2), Render: "🚜 Refactor", Description: "Code change, no behaviour change"},
		{Name: "docs", Order: order(3), Render: "📚 Documentation", Description: "Documentation only"},
		{Name: "perf", Order: order(4), Render: "⚡ Performance", Description: "Performance improvement"},
		{Name: "style", Order: order(5), Render: "🎨 Styling", Description: "Formatting / whitespace"},
		{Name: "test", Order: order(6), Render: "🧪 Testing", Description: "Adding or fixing tests"},
		{Name: "chore", Order: order(7), Render: "⚙️ Miscellaneous Tasks", Description: "Tooling / housekeeping"},
		{Name: "ci", Order: order(7), Render: "⚙️ Miscellaneous Tasks", Description: "CI / release tooling"},
		{Name: "build", Description: "Build system / dependencies"},
	}
}

// EffectiveTypes merges user type rules over the built-in defaults, keyed by name:
//   - a user entry for an existing name replaces that default entry wholesale, so omitting
//     render/order means "no label / unordered", not "inherit the default";
//   - remove:true drops the name from the effective set;
//   - an unknown name is appended after the defaults.
//
// The result is the single effective type set consumed by `heraut commit verify`/`create`
// (the allow-list) and by the native renderer (the section taxonomy). Default order is
// preserved; user-added types follow.
func EffectiveTypes(user []TypeRule) []TypeRule {
	merged := make(map[string]TypeRule)
	order := make([]string, 0)
	for _, t := range defaultTypes() {
		merged[t.Name] = t
		order = append(order, t.Name)
	}
	for _, u := range user {
		_, exists := merged[u.Name]
		if u.Remove {
			delete(merged, u.Name)
			continue
		}
		merged[u.Name] = u
		if !exists {
			order = append(order, u.Name)
		}
	}
	out := make([]TypeRule, 0, len(order))
	for _, name := range order {
		if t, ok := merged[name]; ok {
			out = append(out, t)
		}
	}
	return out
}

// defaultExcludes is the built-in exclude set merged under user rendering.excludes: heraut's
// own release commits plus common dependency / PR-merge noise.
func defaultExcludes() []Exclude {
	return []Exclude{
		{Regex: `^chore\(release\):`},
		{Regex: `^chore\(deps.*\)`},
		{Regex: `^chore\(pr\)`},
		{Regex: `^chore\(pull\)`},
	}
}

// EffectiveExcludes returns the built-in default excludes followed by the user's excludes:
// user entries augment the defaults (they do not replace them).
func EffectiveExcludes(user []Exclude) []Exclude {
	return append(defaultExcludes(), user...)
}

// ScopeRule configures one allowed commit scope, merged over a small built-in default set
// (dependency / release scopes — see defaultScopes) by name, like TypeRule. remove drops a
// default; description is shown beside the scope in the `heraut commit create` wizard.
type ScopeRule struct {
	Name        string `yaml:"name"`
	Remove      bool   `yaml:"remove,omitempty"`
	Description string `yaml:"description,omitempty"`
}

// defaultScopes is the built-in scope set merged under user config: the cross-project,
// tooling-driven scopes (dependabot/renovate dependency bumps, release commits) that also
// align with heraut's default rendering.excludes. Projects add their own scopes over these,
// or drop one with remove:true.
func defaultScopes() []ScopeRule {
	return []ScopeRule{
		{Name: "deps", Description: "Dependency updates"},
		{Name: "deps-dev", Description: "Dev-dependency updates"},
		{Name: "release", Description: "Release / version bumps"},
	}
}

// EffectiveScopes merges user scope rules over the built-in defaults, keyed by name (like
// EffectiveTypes): a listed scope replaces that default's entry, remove:true drops a name, an
// unknown name is appended. The result is the wizard's scope list and — when
// scopes_restricted is true — the verify allow-list.
func EffectiveScopes(user []ScopeRule) []ScopeRule {
	merged := make(map[string]ScopeRule)
	order := make([]string, 0)
	for _, s := range defaultScopes() {
		merged[s.Name] = s
		order = append(order, s.Name)
	}
	for _, u := range user {
		_, exists := merged[u.Name]
		if u.Remove {
			delete(merged, u.Name)
			continue
		}
		merged[u.Name] = u
		if !exists {
			order = append(order, u.Name)
		}
	}
	out := make([]ScopeRule, 0, len(order))
	for _, name := range order {
		if s, ok := merged[name]; ok {
			out = append(out, s)
		}
	}
	return out
}

// ScopeNames returns the names of the given scopes, in order.
func ScopeNames(scopes []ScopeRule) []string {
	names := make([]string, len(scopes))
	for i, s := range scopes {
		names[i] = s.Name
	}
	return names
}
