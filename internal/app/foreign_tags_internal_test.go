package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForeignTagsWarning(t *testing.T) {
	isVersion := func(v string) bool { return v == "1.0.0" || v == "1.1.0" || v == "2.0.0" }
	tests := []struct {
		name   string
		tags   []string
		prefix string
		want   string
	}{
		{name: "the most common prefix wins", tags: []string{"rel-2.0.0", "1.0.0", "1.1.0"}, prefix: "v",
			want: `no tag matches versioning.tag_prefix "v", but 2 tags look like versions with the prefix "" (e.g. 1.0.0): set versioning.tag_prefix: "" to continue from them`},
		{name: "a tie keeps the first seen", tags: []string{"rel-2.0.0", "1.0.0"}, prefix: "v",
			want: `no tag matches versioning.tag_prefix "v", but 1 tag looks like a version with the prefix "rel-" (rel-2.0.0): set versioning.tag_prefix: "rel-" to continue from it`},
		{name: "tags under the prefix are not foreign", tags: []string{"v1.0.0"}, prefix: "v"},
		{name: "no digit, no version", tags: []string{"latest", "deploy"}, prefix: "v"},
		{name: "digits that are no version", tags: []string{"build-42"}, prefix: "v"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, foreignTagsWarning(tc.tags, tc.prefix, isVersion))
		})
	}
}
