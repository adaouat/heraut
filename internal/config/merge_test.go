package config_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeContentDriver(t *testing.T) {
	t.Run("nil base returns override", func(t *testing.T) {
		ovr := &config.ContentDriver{Generator: "git-cliff"}
		got := config.MergeContentDriver(nil, ovr)
		assert.Equal(t, "git-cliff", got.Generator)
	})

	t.Run("nil override returns base", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"}
		got := config.MergeContentDriver(base, nil)
		assert.Equal(t, "CHANGELOG.md", got.Output)
	})

	t.Run("partial override inherits unset fields", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Output: "CHANGELOG.md"}
		ovr := &config.ContentDriver{Config: "cliff.prod.toml"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, "git-cliff", got.Generator, "generator inherited")
		assert.Equal(t, "CHANGELOG.md", got.Output, "output inherited")
		assert.Equal(t, "cliff.prod.toml", got.Config, "config overridden")
	})

	t.Run("same generator merges fields", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Config: "a.toml", Output: "OUT.md"}
		ovr := &config.ContentDriver{Generator: "git-cliff", Config: "b.toml"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, "b.toml", got.Config)
		assert.Equal(t, "OUT.md", got.Output, "inherited")
	})

	t.Run("different generator is full replace", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Config: "cliff.toml", Output: "OUT.md"}
		ovr := &config.ContentDriver{Generator: "communique", Config: "comm.yaml"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, "communique", got.Generator)
		assert.Equal(t, "comm.yaml", got.Config)
		assert.Empty(t, got.Output, "must NOT inherit git-cliff output when switching generator")
	})

	t.Run("override every field", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Config: "a", Output: "o", TagPattern: "p", Template: "t"}
		ovr := &config.ContentDriver{Generator: "git-cliff", Config: "A", Output: "O", TagPattern: "P", Template: "T"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, &config.ContentDriver{Generator: "git-cliff", Config: "A", Output: "O", TagPattern: "P", Template: "T"}, got)
	})

	t.Run("does not mutate base or override", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Output: "OUT.md"}
		ovr := &config.ContentDriver{Config: "c.toml"}
		_ = config.MergeContentDriver(base, ovr)
		assert.Empty(t, base.Config, "base must be untouched")
		assert.Empty(t, ovr.Output, "override must be untouched")
	})

	t.Run("override unset remote inherits base remote", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Remote: &config.Remote{Type: "azure_devops"}}
		ovr := &config.ContentDriver{Config: "c.toml"}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, base.Remote, got.Remote)
	})

	t.Run("override remote replaces base remote", func(t *testing.T) {
		base := &config.ContentDriver{Generator: "git-cliff", Remote: &config.Remote{Type: "azure_devops"}}
		ovr := &config.ContentDriver{Remote: &config.Remote{Type: "github", Repository: "acme/widgets"}}
		got := config.MergeContentDriver(base, ovr)
		assert.Equal(t, ovr.Remote, got.Remote)
	})
}

func TestMergeContentDriver_BothNil(t *testing.T) {
	require.Nil(t, config.MergeContentDriver(nil, nil))
}
