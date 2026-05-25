package app

import (
	"errors"

	"github.com/adaouat/heraut/internal/versioning/perenv"
)

// IsPromotionGuard reports whether err is one of the per-env promotion guards
// (E001/E002/E003 from ADR-0007). The cmd layer uses this to map such failures
// to exit code 4 without importing the perenv package directly.
func IsPromotionGuard(err error) bool {
	return errors.Is(err, perenv.ErrTargetExists) ||
		errors.Is(err, perenv.ErrDestinationAhead) ||
		errors.Is(err, perenv.ErrNoSourceTags)
}
