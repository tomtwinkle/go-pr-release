package release

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

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

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write file %s: %v", path, err)
	}
}

func appendFile(t *testing.T, path string, content string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("open file %s: %v", path, err)
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("append file %s: %v", path, err)
	}
}

func setupRepositoryWithMergedPullRequests(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	originDir := filepath.Join(tempDir, "origin.git")
	workDir := filepath.Join(tempDir, "work")

	runGit(t, tempDir, "init", "--bare", originDir)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", workDir, err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.com")
	writeFile(t, filepath.Join(workDir, "README.md"), "base\n")
	runGit(t, workDir, "add", "README.md")
	runGit(t, workDir, "commit", "-m", "initial")
	runGit(t, workDir, "branch", "-M", "master")
	runGit(t, workDir, "remote", "add", "origin", originDir)
	runGit(t, workDir, "push", "-u", "origin", "master")

	runGit(t, workDir, "checkout", "-b", "staging")
	runGit(t, workDir, "push", "-u", "origin", "staging")

	runGit(t, workDir, "checkout", "-b", "feature1", "master")
	appendFile(t, filepath.Join(workDir, "README.md"), "feature1\n")
	runGit(t, workDir, "add", "README.md")
	runGit(t, workDir, "commit", "-m", "feature1")
	runGit(t, workDir, "push", "origin", "feature1")
	runGit(t, workDir, "push", "origin", "HEAD:refs/pull/1/head")

	runGit(t, workDir, "checkout", "staging")
	runGit(t, workDir, "merge", "--no-ff", "feature1", "-m", "Merge pull request #1")
	runGit(t, workDir, "push", "origin", "staging")

	runGit(t, workDir, "checkout", "-b", "feature2", "master")
	writeFile(t, filepath.Join(workDir, "feature2.txt"), "feature2\n")
	runGit(t, workDir, "add", "feature2.txt")
	runGit(t, workDir, "commit", "-m", "feature2")
	runGit(t, workDir, "push", "origin", "feature2")
	runGit(t, workDir, "push", "origin", "HEAD:refs/pull/2/head")

	runGit(t, workDir, "checkout", "master")
	runGit(t, workDir, "merge", "--no-ff", "feature2", "-m", "Merge pull request #2")
	runGit(t, workDir, "push", "origin", "master")

	runGit(t, workDir, "checkout", "staging")
	runGit(t, workDir, "merge", "--no-ff", "feature2", "-m", "Merge pull request #2 into staging")
	runGit(t, workDir, "push", "origin", "staging")

	runGit(t, workDir, "fetch", "origin")
	return workDir
}

func setupRepositoryWithChainedAndUnrelatedPullRequests(t *testing.T) string {
	t.Helper()

	tempDir := t.TempDir()
	originDir := filepath.Join(tempDir, "origin.git")
	workDir := filepath.Join(tempDir, "work")

	runGit(t, tempDir, "init", "--bare", originDir)
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", workDir, err)
	}

	runGit(t, workDir, "init")
	runGit(t, workDir, "config", "user.name", "Test User")
	runGit(t, workDir, "config", "user.email", "test@example.com")
	writeFile(t, filepath.Join(workDir, "README.md"), "base\n")
	runGit(t, workDir, "add", "README.md")
	runGit(t, workDir, "commit", "-m", "initial")
	runGit(t, workDir, "branch", "-M", "master")
	runGit(t, workDir, "remote", "add", "origin", originDir)
	runGit(t, workDir, "push", "-u", "origin", "master")

	runGit(t, workDir, "checkout", "-b", "staging")
	runGit(t, workDir, "push", "-u", "origin", "staging")

	runGit(t, workDir, "checkout", "-b", "feature1", "master")
	writeFile(t, filepath.Join(workDir, "feature1.txt"), "feature1\n")
	runGit(t, workDir, "add", "feature1.txt")
	runGit(t, workDir, "commit", "-m", "feature1")
	runGit(t, workDir, "push", "origin", "feature1")
	runGit(t, workDir, "push", "origin", "HEAD:refs/pull/1/head")

	runGit(t, workDir, "checkout", "staging")
	runGit(t, workDir, "merge", "--no-ff", "feature1", "-m", "Merge pull request #1")
	runGit(t, workDir, "push", "origin", "staging")

	runGit(t, workDir, "checkout", "-b", "feature2", "feature1")
	writeFile(t, filepath.Join(workDir, "feature2.txt"), "feature2\n")
	runGit(t, workDir, "add", "feature2.txt")
	runGit(t, workDir, "commit", "-m", "feature2")
	runGit(t, workDir, "push", "origin", "feature2")
	runGit(t, workDir, "push", "origin", "HEAD:refs/pull/2/head")

	runGit(t, workDir, "checkout", "staging")
	runGit(t, workDir, "merge", "--no-ff", "feature2", "-m", "Merge pull request #2")
	runGit(t, workDir, "push", "origin", "staging")

	runGit(t, workDir, "checkout", "-b", "feature3", "master")
	writeFile(t, filepath.Join(workDir, "feature3.txt"), "feature3\n")
	runGit(t, workDir, "add", "feature3.txt")
	runGit(t, workDir, "commit", "-m", "feature3")
	runGit(t, workDir, "push", "origin", "feature3")
	runGit(t, workDir, "push", "origin", "HEAD:refs/pull/3/head")

	runGit(t, workDir, "checkout", "-b", "feature4", "master")
	writeFile(t, filepath.Join(workDir, "feature4.txt"), "feature4\n")
	runGit(t, workDir, "add", "feature4.txt")
	runGit(t, workDir, "commit", "-m", "feature4")
	runGit(t, workDir, "push", "origin", "feature4")
	runGit(t, workDir, "push", "origin", "HEAD:refs/pull/4/head")

	runGit(t, workDir, "checkout", "master")
	runGit(t, workDir, "merge", "--no-ff", "feature4", "-m", "Merge pull request #4")
	runGit(t, workDir, "push", "origin", "master")

	runGit(t, workDir, "checkout", "staging")
	runGit(t, workDir, "merge", "--no-ff", "feature4", "-m", "Merge pull request #4 into staging")
	runGit(t, workDir, "push", "origin", "staging")

	runGit(t, workDir, "fetch", "origin")
	return workDir
}
