package gh_test

import (
	"context"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/tomtwinkle/go-pr-release/internal/pkg/gh"
)

func TestGh_GitRemoteConfig(t *testing.T) {
	t.Run("GitRemoteConfig", func(t *testing.T) {
		remoteConfig, err := gh.GitRemoteConfig("../../../.git", "origin")
		assert.NoError(t, err)
		assert.Equal(t, "tomtwinkle", remoteConfig.Owner)
		assert.Equal(t, "go-pr-release", remoteConfig.Repo)
	})
}

func TestGh_GetMergedPRs(t *testing.T) {
	assert.NoError(t, godotenv.Load("../../../.env"))
	t.Run("GetMergedPRs", func(t *testing.T) {
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		owner := "tomtwinkle"
		repo := "go-pr-release-test"
		fromBranch := "develop"
		toBranch := "main"
		ctx := context.Background()

		g := gh.New(ctx, token)
		prs, err := g.GetMergedPRs(ctx, owner, repo, fromBranch, toBranch)
		assert.NoError(t, err)
		wantIDs := []int{9, 6}
		wantTitles := []string{"feat: pr8 can merge", "feat: pr5 can merge"}

		if assert.Equal(t, len(wantIDs), len(prs)) {
			for i, pr := range prs {
				assert.Equal(t, wantIDs[i], pr.GetNumber())
				assert.Equal(t, wantTitles[i], pr.GetTitle())
				t.Logf("%+v,%+v,%+v,%+v,%+v,%+v,%+v", pr.GetID(), pr.GetTitle(), pr.GetState(), pr.MergedAt, pr.GetMergeCommitSHA(), pr.GetUser().GetLogin(), pr.GetURL())
			}
		}
	})
}

func TestGh_CreatePRFromBranch(t *testing.T) {
	assert.NoError(t, godotenv.Load("../../../.env"))
	t.Run("Create PR from branch", func(t *testing.T) {
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		owner := "tomtwinkle"
		repo := "go-pr-release-test"
		ctx := context.Background()

		g := gh.New(ctx, token)
		pr, err := g.CreateReleasePR(ctx, owner, repo, "Merge to main from develop", "develop", "main", "test")
		assert.NoError(t, err)
		t.Logf("%+v", pr)
	})

	t.Run("Edit PR from branch", func(t *testing.T) {
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		owner := "tomtwinkle"
		repo := "go-pr-release-test"
		ctx := context.Background()

		g := gh.New(ctx, token)
		pr, err := g.CreateReleasePR(ctx, owner, repo, "Merge to main from develop", "develop", "main", "test")
		assert.NoError(t, err)
		t.Logf("%+v", pr)
	})
}
