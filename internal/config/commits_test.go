package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func typeNames(types []TypeRule) []string {
	out := make([]string, len(types))
	for i, t := range types {
		out[i] = t.Name
	}
	return out
}

func findType(types []TypeRule, name string) *TypeRule {
	for i := range types {
		if types[i].Name == name {
			return &types[i]
		}
	}
	return nil
}

func findScope(scopes []ScopeRule, name string) *ScopeRule {
	for i := range scopes {
		if scopes[i].Name == name {
			return &scopes[i]
		}
	}
	return nil
}

func TestEffectiveTypes_DefaultsWhenEmpty(t *testing.T) {
	got := EffectiveTypes(nil)
	assert.Equal(t,
		[]string{"feat", "fix", "refactor", "docs", "perf", "style", "test", "chore", "ci", "build"},
		typeNames(got),
		"default commits.types is the ADR-0027 verify type set, render-labeled")

	feat := findType(got, "feat")
	require.NotNil(t, feat)
	require.NotNil(t, feat.Order)
	assert.Equal(t, 0, *feat.Order)
	assert.Equal(t, "🚀 Features", feat.Render)
}

func TestEffectiveTypes_UserEntryReplacesDefaultWholesale(t *testing.T) {
	// Listing a type replaces its default entry: omitting render/order means
	// "no label / unordered", not "inherit the default".
	got := EffectiveTypes([]TypeRule{{Name: "chore"}})

	chore := findType(got, "chore")
	require.NotNil(t, chore)
	assert.Equal(t, "", chore.Render, "listed chore without render keeps render empty (renderer capitalizes)")
	assert.Nil(t, chore.Order, "listed chore without order is unordered")

	// untouched defaults keep their labels
	assert.Equal(t, "🐛 Bug Fixes", findType(got, "fix").Render)
}

func TestEffectiveTypes_AddsUnknownTypeAfterDefaults(t *testing.T) {
	order := 1
	got := EffectiveTypes([]TypeRule{{Name: "deps", Order: &order, Render: "📦 Dependencies"}})

	deps := findType(got, "deps")
	require.NotNil(t, deps)
	assert.Equal(t, "📦 Dependencies", deps.Render)
	assert.Equal(t, "deps", typeNames(got)[len(got)-1], "new types append after the defaults")
}

func TestEffectiveTypes_RemoveDropsDefault(t *testing.T) {
	got := EffectiveTypes([]TypeRule{{Name: "build", Remove: true}})

	assert.Nil(t, findType(got, "build"), "remove:true drops build from the effective set")
	assert.Contains(t, typeNames(got), "feat", "remove does not affect other types")
}

func TestEffectiveTypes_OverrideOrderAndRender(t *testing.T) {
	order := 9
	got := EffectiveTypes([]TypeRule{{Name: "feat", Order: &order, Render: "Features!"}})

	feat := findType(got, "feat")
	require.NotNil(t, feat)
	require.NotNil(t, feat.Order)
	assert.Equal(t, 9, *feat.Order)
	assert.Equal(t, "Features!", feat.Render)
}

func TestDefaultTypes_CarryWizardDescriptions(t *testing.T) {
	feat := findType(EffectiveTypes(nil), "feat")
	require.NotNil(t, feat)
	assert.Equal(t, "A new feature", feat.Description)
}

func TestEffectiveScopes_DefaultsWhenEmpty(t *testing.T) {
	assert.Equal(t, []string{"deps", "deps-dev", "release"}, ScopeNames(EffectiveScopes(nil)))
}

func TestEffectiveScopes_UserMergesOverDefaultsAndRemove(t *testing.T) {
	scopes := []ScopeRule{
		{Name: "cmd"},
		{Name: "config"},
		{Name: "release", Remove: true}, // drop a built-in default scope
	}
	assert.Equal(t, []string{"deps", "deps-dev", "cmd", "config"}, ScopeNames(EffectiveScopes(scopes)))
}

func TestDefaultScopes_CarryDescriptions(t *testing.T) {
	deps := findScope(EffectiveScopes(nil), "deps")
	require.NotNil(t, deps)
	assert.Equal(t, "Dependency updates", deps.Description)
}
