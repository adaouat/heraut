package calver_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/adaouat/forge/exec/exectest"
	"github.com/adaouat/heraut/internal/config"
	"github.com/adaouat/heraut/internal/versioning/calver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func fixedNow(year int, month time.Month, day int) func() time.Time {
	return func() time.Time {
		return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
	}
}

func strPtr(s string) *string { return &s }

func TestResolve_NoTags_FirstRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.0", result.Version)
	assert.Equal(t, "2026.05.0", result.Tag)
	assert.Equal(t, "", result.CurrentTag)

	require.Len(t, mr.Calls, 1)
	assert.Equal(t, "git", mr.Calls[0].Name)
	assert.Equal(t, []string{"tag", "-l", "*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_WithPrefix_NoTags_FirstRelease(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy:  "calver",
			Format:    "YYYY.MM.PATCH",
			TagPrefix: strPtr("v"),
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.0", result.Version)
	assert.Equal(t, "v2026.05.0", result.Tag)
	assert.Equal(t, []string{"tag", "-l", "v*", "--sort=-version:refname"}, mr.Calls[0].Args)
}

func TestResolve_SamePeriod_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.05.1\n2026.05.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.2", result.Version)
	assert.Equal(t, "2026.05.2", result.Tag)
	assert.Equal(t, "2026.05.1", result.CurrentTag)
}

func TestResolve_NewMonth_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.04.5\n2026.04.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 1))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.0", result.Version)
	assert.Equal(t, "2026.04.5", result.CurrentTag)
}

func TestResolve_NewYear_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2025.12.3\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.January, 5))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.01.0", result.Version)
}

func TestResolve_DailyFormat_SameDay_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.05.24.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.DD.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.24.1", result.Version)
}

func TestResolve_DailyFormat_NewDay_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.05.23.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.DD.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.24.0", result.Version)
}

// 2026-05-24 is ISO week 21.
func TestResolve_WeeklyFormat_SameWeek_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.21.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.WW.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.21.1", result.Version)
}

func TestResolve_WeeklyFormat_NewWeek_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.20.3\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.WW.PATCH",
		},
	}

	// 2026-05-24 is ISO week 21; previous tag was week 20 → period change
	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.21.0", result.Version)
}

func TestResolve_QuarterlyFormat_SameQuarter_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.2.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.QQ.PATCH",
		},
	}

	// May = Q2
	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.2.1", result.Version)
}

func TestResolve_QuarterlyFormat_NewQuarter_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.1.5\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.QQ.PATCH",
		},
	}

	// May = Q2, previous tag was Q1
	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.2.0", result.Version)
}

func TestResolve_SemesterFormat_SameSemester_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.1.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.SS.PATCH",
		},
	}

	// May = S1
	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.1.3", result.Version)
}

func TestResolve_SemesterFormat_NewSemester_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.1.5\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.SS.PATCH",
		},
	}

	// July = S2
	r := calver.New(mr, cfg, fixedNow(2026, time.July, 1))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.2.0", result.Version)
}

func TestResolve_SprintFormat_SameSprint_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.5.2\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.SPRINT.PATCH",
			Sprint:   5,
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.5.3", result.Version)
}

func TestResolve_SprintFormat_NewSprint_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.5.3\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.SPRINT.PATCH",
			Sprint:   6,
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.6.0", result.Version)
}

func TestResolve_YearlyFormat_SameYear_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2026.3\n2026.0\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.4", result.Version)
}

func TestResolve_YearlyFormat_NewYear_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("2025.7\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.January, 1))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.0", result.Version)
}

// Tags that don't match the CalVer format are skipped; first parsable tag wins.
func TestResolve_IgnoresUnparsableTags(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("some-unrelated-tag\n2026.05.1\n", "", nil)

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	result, err := r.Resolve()
	require.NoError(t, err)

	assert.Equal(t, "2026.05.2", result.Version)
	assert.Equal(t, "2026.05.1", result.CurrentTag)
}

func TestResolve_GitTagError(t *testing.T) {
	mr := exectest.NewMockRunner()
	mr.QueueResponse("", "fatal: not a git repo", fmt.Errorf("exit 128"))

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	_, err := r.Resolve()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "listing git tags")
}

// BumpFromDate is used by the perenv calculator interface.
func TestBumpFromDate_NoTags_FirstRelease(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	version, err := r.BumpFromDate(nil)
	require.NoError(t, err)
	assert.Equal(t, "2026.05.0", version)
}

func TestBumpFromDate_SamePeriod_PatchIncrement(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 24))
	version, err := r.BumpFromDate([]string{"2026.05.1", "2026.05.0"})
	require.NoError(t, err)
	assert.Equal(t, "2026.05.2", version)
}

func TestBumpFromDate_NewPeriod_PatchReset(t *testing.T) {
	mr := exectest.NewMockRunner()

	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}

	r := calver.New(mr, cfg, fixedNow(2026, time.May, 1))
	version, err := r.BumpFromDate([]string{"2026.04.5"})
	require.NoError(t, err)
	assert.Equal(t, "2026.05.0", version)
}

func TestBumpAuto_Unsupported(t *testing.T) {
	mr := exectest.NewMockRunner()
	cfg := &config.Config{
		Versioning: config.Versioning{
			Strategy: "calver",
			Format:   "YYYY.MM.PATCH",
		},
	}
	r := calver.New(mr, cfg, fixedNow(2026, time.May, 1))
	_, err := r.BumpAuto(nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}
