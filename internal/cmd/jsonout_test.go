package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/adaouat/heraut/internal/exitcode"
	"github.com/adaouat/heraut/internal/versioning/perenv"
)

type hintedErr struct{}

func (hintedErr) Error() string { return "tag format needs a build id" }
func (hintedErr) Hint() string  { return "pass --set-build-id" }

func TestWriteJSON(t *testing.T) {
	type payload struct {
		Tag string `json:"tag"`
	}
	tests := []struct {
		name     string
		data     any
		warnings []string
		err      error
		want     string
	}{
		{"success", payload{Tag: "v1.0.0"}, nil, nil,
			`{"ok":true,"data":{"tag":"v1.0.0"},"warnings":[]}`},
		{"warnings travel in the envelope", payload{Tag: "v1.0.0"}, []string{"ordering"}, nil,
			`{"ok":true,"data":{"tag":"v1.0.0"},"warnings":["ordering"]}`},
		{"unclassified error is a usage error", nil, nil, errors.New("boom"),
			`{"ok":false,"warnings":[],"error":{"code":"usage","message":"boom"}}`},
		{"markup is not escaped", nil, nil, errors.New("a<b & c"),
			`{"ok":false,"warnings":[],"error":{"code":"usage","message":"a<b & c"}}`},
		{"config", nil, nil, exitcode.Wrap(exitcode.Config, errors.New("bad yaml")),
			`{"ok":false,"warnings":[],"error":{"code":"config","message":"bad yaml"}}`},
		{"runtime", nil, nil, exitcode.Wrap(exitcode.Runtime, errors.New("git failed")),
			`{"ok":false,"warnings":[],"error":{"code":"runtime","message":"git failed"}}`},
		{"promotion guard keeps its E-code", nil, nil,
			exitcode.Wrap(exitcode.Promotion, fmt.Errorf("resolving: %w", perenv.ErrTargetExists)),
			`{"ok":false,"warnings":[],"error":{"code":"E001","message":"resolving: E001: target tag already exists"}}`},
		{"a hint is carried", nil, nil, exitcode.Wrap(exitcode.Config, hintedErr{}),
			`{"ok":false,"warnings":[],"error":{"code":"config","message":"tag format needs a build id","hint":"pass --set-build-id"}}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer

			got := writeJSON(&out, tc.data, tc.warnings, tc.err)

			assert.Equal(t, tc.want+"\n", out.String())
			if tc.err == nil {
				require.NoError(t, got)
				return
			}
			assert.Equal(t, tc.err, got, "the error comes back unchanged so the exit code is preserved")
		})
	}
}
