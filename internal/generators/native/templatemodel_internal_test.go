package native

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplateModel_FieldsPresent(t *testing.T) {
	r := tplRelease{
		Version: "1.2.3", Tag: "v1.2.3",
		Groups: []tplGroup{{Name: "Features", Commits: []tplCommit{{
			Description: "x", ShortHash: "abc1234",
			Author:  Author{Username: "jane"},
			PR:      &tplPR{Number: 42, Ref: "#42", Author: Author{Username: "jane"}},
			Footers: []tplFooter{{Token: "Refs", Value: "#1"}},
		}}}},
		Contributors: []tplContributor{{Author: Author{Username: "jane"}}},
		Stats:        tplStats{CommitCount: 1},
		Heraut:       tplHeraut{Version: "0.48.0"},
	}
	assert.Equal(t, "#42", r.Groups[0].Commits[0].PR.Ref)
	assert.Equal(t, "Refs", r.Groups[0].Commits[0].Footers[0].Token)
	assert.Equal(t, 1, r.Stats.CommitCount)
	c := tplChangelog{Releases: []tplRelease{r}, Heraut: tplHeraut{Version: "0.48.0"}}
	assert.Len(t, c.Releases, 1)
}
