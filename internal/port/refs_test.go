package port_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/adaouat/heraut/internal/port"
)

func TestTagNames(t *testing.T) {
	tests := []struct {
		name, in string
		want     []string
	}{
		{"one per line", "v1.1.0\nv1.0.0\n", []string{"v1.1.0", "v1.0.0"}},
		{"blank lines and padding are dropped", "\n  v1.1.0  \n\n", []string{"v1.1.0"}},
		{"a name git would read as an option is dropped", "v1.1.0\n--output=pwned\n-x\nv1.0.0", []string{"v1.1.0", "v1.0.0"}},
		{"a dash inside a name is fine", "build-7\nv1.0.0-rc.1", []string{"build-7", "v1.0.0-rc.1"}},
		{"empty", "", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, port.TagNames(tc.in))
		})
	}
}
