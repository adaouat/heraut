package calver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"equal", "2026.05.1", "2026.05.1", 0},
		{"padding is just a number", "2026.05.1", "2026.5.1", 0},
		{"later month", "2026.10.0", "2026.5.9", 1},
		{"earlier year", "2025.12.3", "2026.01.0", -1},
		{"missing component counts as zero", "2026.05", "2026.05.0", 0},
		{"two-digit patch is not a string compare", "2026.05.10", "2026.05.9", 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, CompareVersions(tc.a, tc.b))
		})
	}
}
