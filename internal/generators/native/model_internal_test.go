package native

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPullRequest_HasReviewFields(t *testing.T) {
	pr := PullRequest{
		Number:    1,
		CreatedAt: time.Unix(100, 0),
		MergedAt:  time.Unix(200, 0),
		MergedBy:  Author{Username: "maintainer"},
		Approvers: []Author{{Username: "alice"}, {Username: "bob"}},
	}
	assert.Equal(t, "maintainer", pr.MergedBy.Username)
	assert.Len(t, pr.Approvers, 2)
	assert.False(t, pr.CreatedAt.IsZero())
	assert.False(t, pr.MergedAt.IsZero())
}
