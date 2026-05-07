package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tomtwinkle/go-pr-release/internal/release"
)

func TestExecuteContextVersion(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := ExecuteContext(context.Background(), CommandOptions{
		Name:      "go-pr-release",
		Version:   "1.2.3",
		Commit:    "abc123",
		Date:      "2026-05-07",
		Args:      []string{"--version"},
		Stdout:    &stdout,
		Stderr:    &stderr,
		LookupEnv: func(string) (string, bool) { return "", false },
	})

	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", exitCode)
	}
	if got := stdout.String(); !strings.Contains(got, "go-pr-release 1.2.3 abc123 [2026-05-07]") {
		t.Fatalf("unexpected version output: %q", got)
	}
	if stderr.Len() != 0 {
		t.Fatalf("expected empty stderr, got %q", stderr.String())
	}
}

func TestResolveConfigUsesLegacyEnvironmentVariables(t *testing.T) {
	t.Parallel()

	workDir := initGitRepository(t, "git@github.com:octo/example.git")
	templatePath := filepath.Join(workDir, "release.tmpl")
	if err := os.WriteFile(templatePath, []byte("Release title\nbody\n"), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	env := map[string]string{
		"GO_PR_RELEASE_TOKEN":     "legacy-token",
		"GO_PR_RELEASE_RELEASE":   "main",
		"GO_PR_RELEASE_DEVELOP":   "develop",
		"GO_PR_RELEASE_TEMPLATE":  "release.tmpl",
		"GO_PR_RELEASE_LABELS":    "release,deploy",
		"GO_PR_RELEASE_REVIEWERS": "alice,bob",
		"GO_PR_RELEASE_DRY_RUN":   "true",
	}

	config, err := resolveConfig(context.Background(), workDir, lookupFromMap(env), parsedArgs{})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}

	if config.Token != "legacy-token" {
		t.Fatalf("unexpected token: %q", config.Token)
	}
	if config.ProductionBranch != "main" {
		t.Fatalf("unexpected production branch: %q", config.ProductionBranch)
	}
	if config.StagingBranch != "develop" {
		t.Fatalf("unexpected staging branch: %q", config.StagingBranch)
	}
	if config.TemplatePath != "release.tmpl" {
		t.Fatalf("unexpected template path: %q", config.TemplatePath)
	}
	if got := strings.Join(config.Labels, ","); got != "release,deploy" {
		t.Fatalf("unexpected labels: %q", got)
	}
	if got := strings.Join(config.ExtraReviewers, ","); got != "alice,bob" {
		t.Fatalf("unexpected reviewers: %q", got)
	}
	if !config.DryRun {
		t.Fatalf("expected dry-run to be true")
	}
	if config.Repository.FullName() != "octo/example" {
		t.Fatalf("unexpected repository: %q", config.Repository.FullName())
	}
}

func TestResolveConfigReadsProjectAndHostAwareGitConfig(t *testing.T) {
	t.Parallel()

	workDir := initGitRepository(t, "ssh://git@ghe.example.com/octo/example.git")
	runGit(t, workDir, "config", "pr-release.ghe.example.com.branch.staging", "integration")
	runGit(t, workDir, "config", "pr-release.ghe.example.com.request-pr-author-review", "true")
	configPath := filepath.Join(workDir, ".git-pr-release")
	runGit(t, workDir, "config", "-f", configPath, "pr-release.token", "project-token")
	runGit(t, workDir, "config", "-f", configPath, "pr-release.branch.production", "production")

	config, err := resolveConfig(context.Background(), workDir, lookupFromMap(nil), parsedArgs{})
	if err != nil {
		t.Fatalf("resolve config: %v", err)
	}

	if config.Token != "project-token" {
		t.Fatalf("unexpected token: %q", config.Token)
	}
	if config.ProductionBranch != "production" {
		t.Fatalf("unexpected production branch: %q", config.ProductionBranch)
	}
	if config.StagingBranch != "integration" {
		t.Fatalf("unexpected staging branch: %q", config.StagingBranch)
	}
	if !config.RequestPRAuthorReview {
		t.Fatalf("expected request-pr-author-review to be true")
	}
	if config.Repository.Host != "ghe.example.com" {
		t.Fatalf("unexpected repository host: %q", config.Repository.Host)
	}
}

func TestExecuteContextReturnsNoPRExitCode(t *testing.T) {
	t.Parallel()

	workDir := initGitRepository(t, "git@github.com:octo/example.git")
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := ExecuteContext(context.Background(), CommandOptions{
		Args:    []string{"--token", "dummy"},
		WorkDir: workDir,
		Stdout:  &stdout,
		Stderr:  &stderr,
		LookupEnv: func(string) (string, bool) {
			return "", false
		},
		NewService: func(config release.Config, stdout io.Writer, stderr io.Writer) serviceRunner {
			return stubService{err: release.ErrNoPullRequestsToRelease}
		},
	})

	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}

type stubService struct {
	err error
}

func (s stubService) Run(context.Context) error {
	return s.err
}

func lookupFromMap(values map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		value, ok := values[key]
		return value, ok
	}
}

func initGitRepository(t *testing.T, remoteURL string) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.name", "Test User")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "remote", "add", "origin", remoteURL)
	return dir
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return strings.TrimSpace(string(output))
}
