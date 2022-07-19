package markdown_test

import (
	"context"
	"os"
	"testing"

	"github.com/tomtwinkle/go-pr-release/internal/pkg/gh"

	"github.com/joho/godotenv"

	"github.com/stretchr/testify/assert"
	"github.com/tomtwinkle/go-pr-release/internal/cli"
	"github.com/tomtwinkle/go-pr-release/internal/markdown"
)

func TestMakePRBody(t *testing.T) {
	assert.NoError(t, godotenv.Load("../../.env"))
	t.Run("make pr body default", func(t *testing.T) {
		ctx := context.Background()
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		m := markdown.NewWithConfig(ctx, token, gh.RemoteConfig{
			Owner: "tomtwinkle",
			Repo:  "go-pr-release-test",
		})
		got, err := m.MakePRBody(ctx, cli.Args{
			ReleaseBranch: "main",
			DevelopBranch: "develop",
			Template:      "",
		})
		assert.NoError(t, err)
		t.Log(got)
	})
}
