package app

import (
	"errors"

	"github.com/adaouat/heraut/internal/versioning/perenv"
	"github.com/adaouat/heraut/internal/versioning/tagfmt"
)

// IsPromotionGuard reports whether err is one of the per-env promotion guards
// (E001/E002/E003 from ADR-0007). The cmd layer uses this to map such failures
// to exit code 4 without importing the perenv package directly.
func IsPromotionGuard(err error) bool {
	return errors.Is(err, perenv.ErrTargetExists) ||
		errors.Is(err, perenv.ErrDestinationAhead) ||
		errors.Is(err, perenv.ErrNoSourceTags)
}

// IsEnvSelection reports whether err is a missing or unknown --env on a per-env strategy. The cmd
// layer maps it to the configuration exit code (T359): which environments exist is defined by
// .heraut.yml, so a name outside it is a configuration problem, not a runtime failure.
func IsEnvSelection(err error) bool {
	var sel *perenv.EnvSelectionError
	return errors.As(err, &sel)
}

// IsBranchMismatch reports whether err is the per-env branch guard refusing to operate an
// environment from the wrong branch. It is distinct from a failure to read the current branch.
func IsBranchMismatch(err error) bool {
	var mismatch *BranchMismatchError
	return errors.As(err, &mismatch)
}

// IsBuildIDRequired reports whether err is a tag-format render failure because the template has a
// {build} token but no build ID was supplied. The cmd layer uses this to map such failures to the
// configuration exit code without importing the tagfmt package directly.
func IsBuildIDRequired(err error) bool {
	return errors.Is(err, tagfmt.ErrBuildIDRequired)
}

// PromotionGuardID returns the guard identifier ("E001", "E002" or "E003") err wraps, or "" when err
// is not a promotion guard. Machine-readable output uses it as the error code.
func PromotionGuardID(err error) string {
	switch {
	case errors.Is(err, perenv.ErrTargetExists):
		return "E001"
	case errors.Is(err, perenv.ErrDestinationAhead):
		return "E002"
	case errors.Is(err, perenv.ErrNoSourceTags):
		return "E003"
	}
	return ""
}
