package markdown_test

import (
	"context"
	"os"
	"testing"

	"github.com/tomtwinkle/go-pr-release/internal/pkg/gh"

	"github.com/stretchr/testify/assert"
	"github.com/tomtwinkle/go-pr-release/internal/markdown"
)

func TestMakePRBody(t *testing.T) {
	t.Run("make pr body default", func(t *testing.T) {
		ctx := context.Background()
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		g := gh.NewWithConfig(ctx, token, gh.RemoteConfig{
			Owner: "tomtwinkle",
			Repo:  "go-pr-release-test",
		})
		prs, err := g.GetMergedPRs(ctx, "develop", "main")
		if !assert.NoError(t, err) {
			return
		}
		got, err := markdown.MakePRBody(prs, "")
		assert.NoError(t, err)
		t.Log(got)
	})
}
