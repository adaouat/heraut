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

// With WW in the format, YYYY is the ISO year: the calendar year would pair 2027 with ISO week 53
// of 2026, or mint 2025.01.0 on 2025-12-29 (ISO week 1 of 2026), a tag that may already exist.
func TestBumpFromDate_ISOYearPairsWithWeek(t *testing.T) {
	tests := []struct {
		name   string
		format string
		tags   []string
		now    time.Time
		want   string
	}{
		{"January days still in week 53 keep the old ISO year", "YYYY.WW.PATCH", []string{"2026.53.1"},
			time.Date(2027, time.January, 1, 12, 0, 0, 0, time.UTC), "2026.53.2"},
		{"late December days already in week 1 take the new ISO year", "YYYY.WW.PATCH", []string{"2025.52.3"},
			time.Date(2025, time.December, 29, 12, 0, 0, 0, time.UTC), "2026.01.0"},
		{"the last Sunday of the old ISO year", "YYYY.WW.PATCH", []string{"2025.52.3"},
			time.Date(2025, time.December, 28, 12, 0, 0, 0, time.UTC), "2025.52.4"},
		{"the first Monday of the new ISO year", "YYYY.WW.PATCH", []string{"2026.53.1"},
			time.Date(2027, time.January, 4, 12, 0, 0, 0, time.UTC), "2027.01.0"},
		{"WW anywhere in the format selects the ISO year", "YYYY.MM.WW.PATCH", []string{"2026.12.53.1"},
			time.Date(2027, time.January, 1, 12, 0, 0, 0, time.UTC), "2026.01.53.0"},
		{"formats without WW keep the calendar year", "YYYY.MM.PATCH", []string{"2026.12.4"},
			time.Date(2027, time.January, 1, 12, 0, 0, 0, time.UTC), "2027.01.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: tc.format}}
			r := calver.New(nil, cfg, func() time.Time { return tc.now })

			got, err := r.BumpFromDate(tc.tags)

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestResolve_BaseTag_Characterization pins the base tag for the supported shapes when git hands
// its tags back in the order it is trusted to (T384).
func TestResolve_BaseTag_Characterization(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		sprint      int
		now         time.Time
		tags        string
		wantVersion string
		wantCurrent string
	}{
		{"YYYY.MM.PATCH, same month", "YYYY.MM.PATCH", 0, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "2026.05.10\n2026.05.9\n2026.04.30\n", "2026.05.11", "2026.05.10"},
		{"YYYY.MM.PATCH, new month", "YYYY.MM.PATCH", 0, time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC), "2026.05.10\n2026.05.9\n", "2026.06.0", "2026.05.10"},
		{"YYYY.WW.PATCH, same week", "YYYY.WW.PATCH", 0, time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC), "2026.21.1\n2026.21.0\n2026.20.4\n", "2026.21.2", "2026.21.1"},
		{"SPRINT.YYYY.MM.PATCH, same sprint", "SPRINT.YYYY.MM.PATCH", 12, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "12.2026.05.1\n12.2026.05.0\n11.2026.05.3\n", "12.2026.05.2", "12.2026.05.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.tags, "", nil)
			cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: tc.format, Sprint: tc.sprint}}

			result, err := calver.New(mr, cfg, func() time.Time { return tc.now }).Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantVersion, result.Version)
			assert.Equal(t, tc.wantCurrent, result.CurrentTag)
		})
	}
}

// TestResolve_BaseTag_IsTheHighestParsedTag covers T384: the base is the highest parsed version, not
// the first line git printed, so an ordering git gets wrong (a leading SPRINT, a reordered list)
// cannot make heraut issue a tag that already exists.
func TestResolve_BaseTag_IsTheHighestParsedTag(t *testing.T) {
	tests := []struct {
		name        string
		format      string
		sprint      int
		now         time.Time
		tags        string
		wantVersion string
		wantCurrent string
	}{
		{"YYYY.MM.PATCH, highest not first", "YYYY.MM.PATCH", 0, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "2026.05.0\n2026.05.2\n2026.05.1\n", "2026.05.3", "2026.05.2"},
		{"YYYY.MM.PATCH, older month printed first", "YYYY.MM.PATCH", 0, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "2026.04.9\n2026.05.1\n", "2026.05.2", "2026.05.1"},
		{"YYYY.WW.PATCH, highest not first", "YYYY.WW.PATCH", 0, time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC), "2026.20.7\n2026.21.1\n2026.21.0\n", "2026.21.2", "2026.21.1"},
		{"SPRINT.YYYY.MM.PATCH, newest sprint printed last", "SPRINT.YYYY.MM.PATCH", 12, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "11.2026.05.9\n12.2026.05.1\n", "12.2026.05.2", "12.2026.05.1"},
		{"a tag that does not parse is ignored", "YYYY.MM.PATCH", 0, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "nightly\n2026.05.0\n2026.05.1\n", "2026.05.2", "2026.05.1"},
		{"equal values keep git's first", "YYYY.MM.PATCH", 0, time.Date(2026, 5, 24, 12, 0, 0, 0, time.UTC), "2026.05.1\n2026.5.1\n", "2026.05.2", "2026.05.1"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse(tc.tags, "", nil)
			cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: tc.format, Sprint: tc.sprint}}

			result, err := calver.New(mr, cfg, func() time.Time { return tc.now }).Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.wantVersion, result.Version)
			assert.Equal(t, tc.wantCurrent, result.CurrentTag)
		})
	}
}

// TestBumpFromDate_BaseIsTheHighestParsedVersion is the same rule for the per-env entry point.
func TestBumpFromDate_BaseIsTheHighestParsedVersion(t *testing.T) {
	cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver-per-env", Format: "YYYY.MM.PATCH"}}
	r := calver.New(nil, cfg, fixedNow(2026, time.May, 24))

	next, err := r.BumpFromDate([]string{"2026.05.0", "2026.05.2", "2026.05.1"})
	require.NoError(t, err)
	assert.Equal(t, "2026.05.3", next)
}

// TestResolve_DatesAreUTC covers T385: the calendar fields come from the UTC instant, so the same
// moment gives the same version on a laptop in any zone and on a CI runner.
func TestResolve_DatesAreUTC(t *testing.T) {
	minus5 := time.FixedZone("UTC-5", -5*3600)
	plus9 := time.FixedZone("UTC+9", 9*3600)
	tests := []struct {
		name   string
		format string
		now    time.Time
		want   string
	}{
		{"month rolls over in UTC first", "YYYY.MM.PATCH", time.Date(2026, 5, 31, 23, 30, 0, 0, minus5), "2026.06.0"},
		{"day rolls over in UTC first", "YYYY.MM.DD.PATCH", time.Date(2026, 5, 24, 22, 0, 0, 0, minus5), "2026.05.25.0"},
		{"local date is already ahead of UTC", "YYYY.MM.DD.PATCH", time.Date(2026, 6, 1, 6, 0, 0, 0, plus9), "2026.05.31.0"},
		{"ISO week rolls over in UTC first", "YYYY.WW.PATCH", time.Date(2026, 5, 24, 23, 30, 0, 0, minus5), "2026.22.0"},
		{"quarter rolls over in UTC first", "YYYY.QQ.PATCH", time.Date(2026, 3, 31, 22, 0, 0, 0, minus5), "2026.2.0"},
		{"semester rolls over in UTC first", "YYYY.SS.PATCH", time.Date(2026, 6, 30, 22, 0, 0, 0, minus5), "2026.2.0"},
		{"year rolls over in UTC first", "YYYY.PATCH", time.Date(2026, 12, 31, 20, 0, 0, 0, minus5), "2027.0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mr := exectest.NewMockRunner()
			mr.QueueResponse("", "", nil)
			cfg := &config.Config{Versioning: config.Versioning{Strategy: "calver", Format: tc.format}}

			result, err := calver.New(mr, cfg, func() time.Time { return tc.now }).Resolve()
			require.NoError(t, err)
			assert.Equal(t, tc.want, result.Version)
		})
	}
}
