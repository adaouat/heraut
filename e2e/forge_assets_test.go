//go:build e2e_forge

package e2e_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/e2e/forgeharness"
	"github.com/adaouat/heraut/e2e/harness"
)

// ADR-0076 on the real forges: a release asset pattern must match a file.
const assetTarget = "      assets:\n        - \"dist/*.txt\"\n"

func TestForge_B7_AssetGate(t *testing.T) {
	bin := harness.Binary(t)

	t.Run("a pattern matching nothing is refused before anything reaches the forge", func(t *testing.T) {
		eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
			commitConfigAndSubjects(ws, ws.Config(semverBlock(ws), assetTarget), "feat: one")
			head := ws.Repo.Git("rev-parse", "HEAD")

			res := ws.Run(bin, ws.Env(), "release", "--offline")

			require.Equal(t, exitRuntime, res.ExitCode, "stdout:\n%s\nstderr:\n%s", res.Stdout, res.Stderr)
			assert.Contains(t, normalize(res.Stdout+" "+res.Stderr), `"dist/*.txt" matches no file`)
			assert.Equal(t, head, ws.Repo.Git("rev-parse", "HEAD"), "no changelog commit")
			assert.Empty(t, ws.Repo.Git("tag", "-l"), "no local tag")
			tags, err := ws.Forge.Tags(ws.TagPrefix)
			require.NoError(t, err)
			assert.Empty(t, tags, "no tag reached the forge")
			_, ok, err := ws.Forge.Release(ws.TagPrefix + "0.1.0")
			require.NoError(t, err)
			assert.False(t, ok, "no release was created")
		})
	})

	t.Run("allow_missing_assets publishes without the asset", func(t *testing.T) {
		eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
			commitConfigAndSubjects(ws, ws.Config(semverBlock(ws), assetTarget+"  allow_missing_assets: true\n"), "feat: one")

			release(t, ws, bin)

			rel, ok, err := ws.Forge.Release(ws.TagPrefix + "0.1.0")
			require.NoError(t, err)
			require.True(t, ok, "the release exists")
			assert.Equal(t, 0, rel.AssetCount, "nothing to attach")
		})
	})

	t.Run("matching files are attached", func(t *testing.T) {
		eachForge(t, func(t *testing.T, ws *forgeharness.Workspace) {
			ws.Repo.WriteFile("dist/a.txt", "a\n")
			ws.Repo.WriteFile("dist/b.txt", "b\n")
			ws.Repo.Git("add", "dist")
			commitConfigAndSubjects(ws, ws.Config(semverBlock(ws), assetTarget), "feat: one")

			release(t, ws, bin)

			rel, ok, err := ws.Forge.Release(ws.TagPrefix + "0.1.0")
			require.NoError(t, err)
			require.True(t, ok)
			assert.Equal(t, 2, rel.AssetCount, "both files were attached")
		})
	})
}
