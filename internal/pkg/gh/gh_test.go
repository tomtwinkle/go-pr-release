package gh_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"

	"github.com/tomtwinkle/go-pr-release/internal/pkg/gh"
)

func TestGh_GetMergedPRs(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok {
		t.SkipNow()
	}
	assert.NoError(t, godotenv.Load("../../../.env"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

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

		g, err := gh.NewWithConfig(ctx, token, logger, gh.RemoteConfig{Owner: owner, Repo: repo})
		assert.NoError(t, err)
		prs, err := g.GetMergedPRs(ctx, fromBranch, toBranch)
		assert.NoError(t, err)
		wantIDs := []int{9, 6}
		wantTitles := []string{"feat: pr8 can merge", "feat: pr5 can merge"}

		if assert.Equal(t, len(wantIDs), len(prs)) {
			for i, pr := range prs {
				assert.Equal(t, wantIDs[i], pr.GetNumber())
				assert.Equal(t, wantTitles[i], pr.GetTitle())
				t.Logf("%+v,%+v,%+v,%+v,%+v,%+v,%+v", pr.GetID(), pr.GetTitle(), pr.GetState(), pr.MergedAt, pr.GetMergeCommitSHA(), pr.GetUser().GetHTMLURL(), pr.GetURL())
			}
		}
	})
}

func TestGh_GetReleasePR(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok {
		t.SkipNow()
	}
	assert.NoError(t, godotenv.Load("../../../.env"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Run("Create PR from branch", func(t *testing.T) {
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		owner := "tomtwinkle"
		repo := "go-pr-release-test"
		ctx := context.Background()

		g, err := gh.NewWithConfig(ctx, token, logger, gh.RemoteConfig{Owner: owner, Repo: repo})
		assert.NoError(t, err)
		pr, err := g.GetReleasePR(ctx, "develop", "main")
		assert.NoError(t, err)
		t.Logf("%+v", pr)
	})
}

func TestGh_CreatePRFromBranch(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok {
		t.SkipNow()
	}
	assert.NoError(t, godotenv.Load("../../../.env"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Run("Create PR from branch", func(t *testing.T) {
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		owner := "tomtwinkle"
		repo := "go-pr-release-test"
		ctx := context.Background()

		g, err := gh.NewWithConfig(ctx, token, logger, gh.RemoteConfig{Owner: owner, Repo: repo})
		assert.NoError(t, err)
		pr, err := g.CreateReleasePR(ctx, "Merge to main from develop", "develop", "main", "test")
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

		g, err := gh.NewWithConfig(ctx, token, logger, gh.RemoteConfig{Owner: owner, Repo: repo})
		assert.NoError(t, err)
		pr, err := g.CreateReleasePR(ctx, "Merge to main from develop", "develop", "main", "test")
		assert.NoError(t, err)
		t.Logf("%+v", pr)
	})
}

func TestGh_AssignReviews(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok {
		t.SkipNow()
	}
	assert.NoError(t, godotenv.Load("../../../.env"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Run("AssignReviews", func(t *testing.T) {
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		owner := "tomtwinkle"
		repo := "go-pr-release-test"
		ctx := context.Background()

		g, err := gh.NewWithConfig(ctx, token, logger, gh.RemoteConfig{Owner: owner, Repo: repo})
		assert.NoError(t, err)
		pr, err := g.CreateReleasePR(ctx, "Merge to main from develop", "develop", "main", "test")
		if !assert.NoError(t, err) {
			return
		}
		pr, err = g.AssignReviews(ctx, pr.GetNumber(), "soe-j")
		assert.NoError(t, err)
		t.Logf("%+v", pr.Assignees)
	})
}

func TestGh_Labeling(t *testing.T) {
	if _, ok := os.LookupEnv("CI"); ok {
		t.SkipNow()
	}
	assert.NoError(t, godotenv.Load("../../../.env"))
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	t.Run("Labeling", func(t *testing.T) {
		token, ok := os.LookupEnv("GO_PR_RELEASE_TOKEN")
		if !assert.True(t, ok) {
			return
		}
		owner := "tomtwinkle"
		repo := "go-pr-release-test"
		ctx := context.Background()

		g, err := gh.NewWithConfig(ctx, token, logger, gh.RemoteConfig{Owner: owner, Repo: repo})
		assert.NoError(t, err)
		pr, err := g.CreateReleasePR(ctx, "Merge to main from develop", "develop", "main", "test")
		if !assert.NoError(t, err) {
			return
		}
		labels, err := g.Labeling(ctx, pr.GetNumber(), "test", "release", "develop")
		assert.NoError(t, err)
		t.Logf("%+v", labels)
	})
}
