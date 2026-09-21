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

// IsBuildIDRequired reports whether err is a tag-format render failure because the template has a
// {build} token but no build ID was supplied. The cmd layer uses this to map such failures to the
// configuration exit code without importing the tagfmt package directly.
func IsBuildIDRequired(err error) bool {
	return errors.Is(err, tagfmt.ErrBuildIDRequired)
}
