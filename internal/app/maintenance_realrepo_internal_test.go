package app

import (
	"strings"
	"testing"

	execadapter "github.com/adaouat/forge/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/generators/native"
	"github.com/adaouat/heraut/internal/versioning/semver"
)

// TestMaintenanceCurrentAndPreRelease_RealRepo replays ADR-0065 worked examples #9 and #10:
// main carries v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0, release/1.3 forks at v1.3.1 with one fix.
func TestMaintenanceCurrentAndPreRelease_RealRepo(t *testing.T) {
	git := historyRepo(t)
	commit := func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag := func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: initial")
	tag("v1.3.0")
	commit("fix: first")
	tag("v1.3.1")
	git("branch", "release/1.3")
	commit("feat: second")
	tag("v1.4.0")
	commit("feat!: third")
	tag("v2.0.0")
	git("checkout", "release/1.3")
	commit("fix: x")

	cfg := &config.Config{Version: "1", Versioning: config.Versioning{
		Strategy: "semver",
		Branches: []config.BranchRule{{Name: "main"}, {Name: "release/*"}},
	}}
	runner := execadapter.New(false, false)

	t.Run("#9 version current", func(t *testing.T) {
		cur, err := CurrentTag(runner, cfg, "", false)
		require.NoError(t, err)
		assert.Equal(t, "v1.3.1", cur)
	})

	t.Run("#10 --pre-release rc", func(t *testing.T) {
		r, err := NewResolver(cfg, "", false, "", "", runner, WithPreRelease("rc"))
		require.NoError(t, err)
		res, err := r.Resolve()
		require.NoError(t, err)
		assert.Equal(t, "v1.3.2-rc.1", res.Tag)
		assert.Equal(t, "v1.3.1", res.CurrentTag)
		tag(res.Tag)

		cur, err := CurrentTag(runner, cfg, "", true)
		require.NoError(t, err)
		assert.Equal(t, "v1.3.2-rc.1", cur)
	})

	t.Run("main unchanged", func(t *testing.T) {
		git("checkout", "main")
		cur, err := CurrentTag(runner, cfg, "", false)
		require.NoError(t, err)
		assert.Equal(t, "v2.0.0", cur)
	})
}

// maintenanceFixture builds the ADR-0065 worked-example history: main carries
// v1.3.0 → v1.3.1 → v1.4.0 → v2.0.0, release/1.3 forks at v1.3.1 and release/1.x at v1.4.0.
// main is left checked out.
func maintenanceFixture(t *testing.T) (git func(args ...string), commit func(msg string), cfg *config.Config) {
	t.Helper()
	git = historyRepo(t)
	commit = func(msg string) { git("commit", "--allow-empty", "-m", msg) }
	tag := func(name string) { git("tag", "-a", "-m", name, name) }

	commit("feat: initial")
	tag("v1.3.0")
	commit("fix: first")
	tag("v1.3.1")
	git("branch", "release/1.3")
	commit("feat: second")
	tag("v1.4.0")
	git("branch", "release/1.x")
	commit("feat!: third")
	tag("v2.0.0")

	cfg = &config.Config{Version: "1", Versioning: config.Versioning{
		Strategy: "semver",
		Branches: []config.BranchRule{{Name: "main"}, {Name: "release/*"}},
	}}
	return git, commit, cfg
}

func resolveOn(t *testing.T, cfg *config.Config, versionOverride string) (string, error) {
	t.Helper()
	r, err := NewResolver(cfg, "", false, versionOverride, "", execadapter.New(false, false))
	if err != nil {
		return "", err
	}
	res, err := r.Resolve()
	return res.Tag, err
}

// TestMaintenanceBranches_RealRepo replays the ADR-0065 worked examples through the production
// wiring against a real git repo.
func TestMaintenanceBranches_RealRepo(t *testing.T) {
	t.Run("#1 fix on release/1.3 is v1.3.2 with notes v1.3.1..v1.3.2", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.3")
		commit("fix: x")

		tag, err := resolveOn(t, cfg, "")
		require.NoError(t, err)
		assert.Equal(t, "v1.3.2", tag)

		git("tag", "-a", "-m", tag, tag)
		driver := config.ContentDriver{}
		gen := buildGenerator(execadapter.New(false, false), &driver, native.ModeReleaseNotes, "", false, false, nil, "", tagOrderFor(cfg, ""))
		notes, err := gen.Generate(tag, historyLC)
		require.NoError(t, err)
		assert.Contains(t, notes, "1 commit(s) contributed", "the range v1.3.1..v1.3.2 holds exactly one commit")
		assert.Contains(t, notes, "X")
		assert.NotContains(t, notes, "First")
		assert.NotContains(t, notes, "Second")
	})

	t.Run("#2 feat on release/1.3 is out of range", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.3")
		commit("feat: y")
		_, err := resolveOn(t, cfg, "")
		require.ErrorIs(t, err, semver.ErrOutOfRange)
	})

	t.Run("#3 feat on release/1.x is v1.5.0", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.x")
		commit("feat: z")
		tag, err := resolveOn(t, cfg, "")
		require.NoError(t, err)
		assert.Equal(t, "v1.5.0", tag)
	})

	t.Run("#4 breaking change on release/1.x is out of range", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.x")
		commit("feat!: b")
		_, err := resolveOn(t, cfg, "")
		require.ErrorIs(t, err, semver.ErrOutOfRange)
	})

	t.Run("#5 main is unaffected by a tag on release/1.3", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.3")
		commit("fix: maintenance")
		git("tag", "-a", "-m", "v1.3.2", "v1.3.2")
		git("checkout", "main")
		commit("fix: w")
		tag, err := resolveOn(t, cfg, "")
		require.NoError(t, err)
		assert.Equal(t, "v2.0.1", tag)
	})

	t.Run("#6 tag already on a side branch collides", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.3")
		git("checkout", "-b", "side")
		commit("fix: elsewhere")
		git("tag", "-a", "-m", "v1.3.2", "v1.3.2")
		git("checkout", "release/1.3")
		commit("fix: x")
		_, err := resolveOn(t, cfg, "")
		require.ErrorIs(t, err, semver.ErrTagExists)
	})

	t.Run("#6 build-metadata release of the version collides, a pre-release does not", func(t *testing.T) {
		for _, tc := range []struct {
			sideTag string
			wantErr bool
		}{
			{sideTag: "v1.3.2+7", wantErr: true},
			{sideTag: "v1.3.2-rc.1"},
		} {
			t.Run(tc.sideTag, func(t *testing.T) {
				git, commit, cfg := maintenanceFixture(t)
				git("checkout", "release/1.3")
				git("checkout", "-b", "side")
				commit("fix: elsewhere")
				git("tag", "-a", "-m", tc.sideTag, tc.sideTag)
				git("checkout", "release/1.3")
				commit("fix: x")

				for _, override := range []string{"", "1.3.2"} {
					tag, err := resolveOn(t, cfg, override)
					if tc.wantErr {
						require.ErrorIs(t, err, semver.ErrTagExists, "override %q", override)
						assert.ErrorContains(t, err, tc.sideTag)
						continue
					}
					require.NoError(t, err, "override %q", override)
					assert.Equal(t, "v1.3.2", tag)
				}
			})
		}
	})

	t.Run("#7 detached HEAD with CI branch variable", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.3")
		commit("fix: x")
		git("checkout", "--detach", "release/1.3")
		t.Setenv("CI_COMMIT_BRANCH", "release/1.3")
		tag, err := resolveOn(t, cfg, "")
		require.NoError(t, err)
		assert.Equal(t, "v1.3.2", tag)
	})

	t.Run("#8 unlisted branch refuses publishing, --force bypasses, preview unchanged", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "-b", "feature/foo")
		commit("fix: foo")
		runner := execadapter.New(false, false)

		require.ErrorIs(t, CheckReleaseBranch(runner, cfg, false), ErrUnlistedBranch)
		require.NoError(t, CheckReleaseBranch(runner, cfg, true))

		tag, err := resolveOn(t, cfg, "")
		require.NoError(t, err)
		assert.Equal(t, "v2.0.1", tag)
	})

	t.Run("#11 underivable range: --set-version passes, auto resolution errors", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "-b", "release/7.8.0")
		commit("fix: x")

		tag, err := resolveOn(t, cfg, "7.8.0")
		require.NoError(t, err)
		assert.Equal(t, "v7.8.0", tag)

		_, err = resolveOn(t, cfg, "")
		require.ErrorIs(t, err, ErrUnderivableRange)
		assert.Contains(t, err.Error(), "release/7.8.0")
	})

	t.Run("regenerated changelog on release/1.3 holds only its own history", func(t *testing.T) {
		git, commit, cfg := maintenanceFixture(t)
		git("checkout", "release/1.3")
		commit("fix: x")
		git("tag", "-a", "-m", "v1.3.2", "v1.3.2")

		body := regenerateSemver(t, cfg, "", config.ContentDriver{}, "v1.3.2")

		var anchors []string
		for _, line := range strings.Split(body, "\n") {
			if strings.HasPrefix(line, "<!-- heraut-release: ") {
				anchors = append(anchors, line)
			}
		}
		assert.Equal(t, []string{
			"<!-- heraut-release: v1.3.2 -->",
			"<!-- heraut-release: v1.3.1 -->",
			"<!-- heraut-release: v1.3.0 -->",
		}, anchors)
		assert.NotContains(t, body, "[v1.4.0]")
		assert.NotContains(t, body, "[v2.0.0]")
	})
}
