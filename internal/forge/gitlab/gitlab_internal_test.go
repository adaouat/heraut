package gitlab

import (
	"testing"

	"github.com/adaouat/heraut/internal/port"
	"github.com/stretchr/testify/assert"
)

func TestGitAuthors_PrefersEmailLocalPart(t *testing.T) {
	got := gitAuthors([]port.Commit{
		{Hash: "aaa", Author: "Alice Smith", Email: "alice@example.com"}, // local-part wins over the name
		{Hash: "bbb", Author: "Bob", Email: ""},                          // no email → git name
		{Hash: "ccc", Author: "", Email: "carol@example.com"},            // no name → local-part
		{Hash: "ddd", Author: "", Email: ""},                             // nothing → omitted
	})
	assert.Equal(t, map[string]string{
		"aaa": "alice",
		"bbb": "Bob",
		"ccc": "carol",
	}, got, "an @handle must not contain spaces when an email local-part is available")
}
