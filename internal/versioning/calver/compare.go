package calver

import (
	"strconv"
	"strings"
)

// CompareVersions compares two dot-separated version strings component by component, reading each
// as an integer (zero padding is just a number; a missing or non-numeric component counts as 0).
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
func CompareVersions(a, b string) int {
	aParts := strings.Split(a, ".")
	bParts := strings.Split(b, ".")

	n := max(len(aParts), len(bParts))
	for i := range n {
		var av, bv int
		if i < len(aParts) {
			av, _ = strconv.Atoi(aParts[i])
		}
		if i < len(bParts) {
			bv, _ = strconv.Atoi(bParts[i])
		}
		switch {
		case av < bv:
			return -1
		case av > bv:
			return 1
		}
	}
	return 0
}

// compareValues orders two parsed versions chronologically: largest calendar unit first, then the
// sprint, then the patch. A dimension the format does not carry is zero on both sides, so it never
// decides. Returns -1 if a < b, 0 if a == b, 1 if a > b.
func compareValues(a, b Values) int {
	pairs := [][2]int{
		{a.Year, b.Year}, {a.Semester, b.Semester}, {a.Quarter, b.Quarter},
		{a.Month, b.Month}, {a.Week, b.Week}, {a.Day, b.Day},
		{a.Sprint, b.Sprint}, {a.Patch, b.Patch},
	}
	for _, p := range pairs {
		switch {
		case p[0] < p[1]:
			return -1
		case p[0] > p[1]:
			return 1
		}
	}
	return 0
}

// CompareByFormat returns a comparison of two bare versions read through format, chronological as
// compareValues orders them. A version that does not parse under the format sorts below every one
// that does, and two such versions compare equal. An invalid format yields an error.
func CompareByFormat(format string) (func(a, b string) int, error) {
	tokens, err := ParseFormat(format)
	if err != nil {
		return nil, err
	}
	return func(a, b string) int {
		va, errA := ParseVersion(tokens, a)
		vb, errB := ParseVersion(tokens, b)
		switch {
		case errA != nil && errB != nil:
			return 0
		case errA != nil:
			return -1
		case errB != nil:
			return 1
		}
		return compareValues(va, vb)
	}, nil
}
