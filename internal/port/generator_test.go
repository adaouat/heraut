package port_test

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

func TestURLTag(t *testing.T) {
	tests := []struct{ in, want string }{
		{"v1.2.3", "v1.2.3"},
		{"v1.4.0+158404", "v1.4.0%2B158404"},
		{"uat/7.4.1+158404", "uat/7.4.1%2B158404"}, // "/" stays literal
		{"v1.4.0-rc.1+a+b", "v1.4.0-rc.1%2Ba%2Bb"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) { assert.Equal(t, tc.want, port.URLTag(tc.in)) })
	}
}
