package markdown_test

import (
	"testing"
	"time"

	"github.com/bxcodec/faker/v3"
	"github.com/google/go-github/v45/github"
	"github.com/stretchr/testify/assert"
	"github.com/tomtwinkle/go-pr-release/internal/markdown"
)

func TestMakePRBody(t *testing.T) {
	t.Run("make pr body default", func(t *testing.T) {
		now := time.Now()
		prs := []*github.PullRequest{
			{
				Number:    github.Int(1),
				Title:     github.String(faker.Name()),
				CreatedAt: &now,
				UpdatedAt: &now,
				MergedAt:  &now,
				Labels:    nil,
				User: &github.User{
					Login:     github.String(faker.Name()),
					AvatarURL: github.String("https://example.com"),
					HTMLURL:   github.String("https://example.com"),
				},
				HTMLURL:            github.String("https://example.com/1"),
				Assignees:          nil,
				RequestedReviewers: nil,
			},
			{
				Number:    github.Int(2),
				Title:     github.String(faker.Name()),
				CreatedAt: &now,
				UpdatedAt: &now,
				MergedAt:  &now,
				Labels:    nil,
				User: &github.User{
					Login:     github.String(faker.Name()),
					AvatarURL: github.String("https://example.com"),
					HTMLURL:   github.String("https://example.com"),
				},
				HTMLURL:            github.String("https://example.com/1"),
				Assignees:          nil,
				RequestedReviewers: nil,
			},
		}
		got, err := markdown.MakePRBody(prs, "")
		assert.NoError(t, err)
		t.Log(got)
	})
}
