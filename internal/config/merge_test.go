package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeContentDriver(t *testing.T) {
	t.Run("nil base returns override", func(t *testing.T) {
		ovr := &config.ContentDriver{}
		got := config.MergeContentDriver(nil, ovr)
		assert.Same(t, ovr, got)
	})

	t.Run("nil override returns base", func(t *testing.T) {
		base := &config.ContentDriver{Output: "CHANGELOG.md"}
		got := config.MergeContentDriver(base, nil)
		assert.Equal(t, "CHANGELOG.md", got.Output)
	})

	t.Run("partial override inherits unset fields", func(t *testing.T) {
		base := &config.ContentDriver{Output: "CHANGELOG.md"}
		ovr := &config.ContentDriver{TagPattern: "v{version}"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, "CHANGELOG.md", got.Output, "output inherited")
		assert.Equal(t, "v{version}", got.TagPattern, "tag pattern overridden")
	})

	t.Run("same generator merges fields", func(t *testing.T) {
		base := &config.ContentDriver{Template: "a.tmpl", Output: "OUT.md"}
		ovr := &config.ContentDriver{Template: "b.tmpl"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, "b.tmpl", got.Template)
		assert.Equal(t, "OUT.md", got.Output, "inherited")
	})

	t.Run("override every field", func(t *testing.T) {
		base := &config.ContentDriver{Output: "o", TagPattern: "p", Template: "t"}
		ovr := &config.ContentDriver{Output: "O", TagPattern: "P", Template: "T"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, &config.ContentDriver{Output: "O", TagPattern: "P", Template: "T"}, got)
	})

	t.Run("does not mutate base or override", func(t *testing.T) {
		base := &config.ContentDriver{Output: "OUT.md"}
		ovr := &config.ContentDriver{TagPattern: "p.tmpl"}
		_ = config.MergeContentDriver(base, ovr)
		assert.Empty(t, base.TagPattern, "base must be untouched")
		assert.Empty(t, ovr.Output, "override must be untouched")
	})

}

func TestMergeContentDriver_BothNil(t *testing.T) {
	require.Nil(t, config.MergeContentDriver(nil, nil))
}

func TestMergeContentDriver_Rendering(t *testing.T) {
	t.Run("templates merge key-by-key, override wins", func(t *testing.T) {
		base := &config.ContentDriver{
			Rendering: &config.Rendering{Templates: map[string]string{"commit": "base-commit", "group": "base-group"}},
		}
		ovr := &config.ContentDriver{
			Rendering: &config.Rendering{Templates: map[string]string{"commit": "env-commit"}},
		}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, "env-commit", got.Rendering.Templates["commit"], "override wins per key")
		assert.Equal(t, "base-group", got.Rendering.Templates["group"], "unset key inherits")
		assert.Equal(t, "base-commit", base.Rendering.Templates["commit"], "base is not mutated")
	})

	t.Run("nil override rendering inherits base", func(t *testing.T) {
		base := &config.ContentDriver{
			Rendering: &config.Rendering{Templates: map[string]string{"commit": "base-commit"}},
		}
		got := config.MergeContentDriver(base, &config.ContentDriver{})
		require.NotNil(t, got.Rendering)
		assert.Equal(t, "base-commit", got.Rendering.Templates["commit"])
	})

	t.Run("override excludes replace base excludes", func(t *testing.T) {
		base := &config.ContentDriver{
			Rendering: &config.Rendering{Excludes: []config.Exclude{{Type: "chore"}}},
		}
		ovr := &config.ContentDriver{
			Rendering: &config.Rendering{Excludes: []config.Exclude{{Type: "ci"}}},
		}
		got := config.MergeContentDriver(base, ovr)
		require.Len(t, got.Rendering.Excludes, 1)
		assert.Equal(t, "ci", got.Rendering.Excludes[0].Type)
	})
}
