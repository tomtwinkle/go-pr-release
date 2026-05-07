package release

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseRemoteURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		rawURL string
		want   Repository
	}{
		{
			name:   "github https",
			rawURL: "https://github.com/motemen/git-pr-release",
			want:   Repository{Scheme: "https", Owner: "motemen", Name: "git-pr-release"},
		},
		{
			name:   "github ssh",
			rawURL: "git@github.com:motemen/git-pr-release.git",
			want:   Repository{Scheme: "https", Owner: "motemen", Name: "git-pr-release"},
		},
		{
			name:   "enterprise http",
			rawURL: "http://ghe.example.com/motemen/git-pr-release",
			want:   Repository{Host: "ghe.example.com", Scheme: "http", Owner: "motemen", Name: "git-pr-release"},
		},
		{
			name:   "enterprise ssh",
			rawURL: "ssh://git@ghe.example.com/motemen/git-pr-release.git",
			want:   Repository{Host: "ghe.example.com", Scheme: "https", Owner: "motemen", Name: "git-pr-release"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRemoteURL(tt.rawURL)
			if err != nil {
				t.Fatalf("parse remote url: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestLookupProjectConfig(t *testing.T) {
	t.Parallel()

	workDir := t.TempDir()
	runGit(t, workDir, "init")
	runGit(t, workDir, "remote", "add", "origin", "ssh://git@ghe.example.com/octo/example.git")
	runGit(t, workDir, "config", "pr-release.ghe.example.com.branch.staging", "integration")
	configPath := filepath.Join(workDir, ".git-pr-release")
	runGit(t, workDir, "config", "-f", configPath, "pr-release.branch.production", "production")

	git := NewGit(workDir)
	repo, err := git.ResolveRemote(context.Background(), DefaultRemoteName)
	if err != nil {
		t.Fatalf("resolve remote: %v", err)
	}

	production, ok, err := git.LookupProjectConfig(context.Background(), repo, "branch.production")
	if err != nil {
		t.Fatalf("lookup project config: %v", err)
	}
	if !ok || production != "production" {
		t.Fatalf("unexpected production branch config: %q (ok=%v)", production, ok)
	}

	staging, ok, err := git.LookupProjectConfig(context.Background(), repo, "branch.staging")
	if err != nil {
		t.Fatalf("lookup host-aware config: %v", err)
	}
	if !ok || staging != "integration" {
		t.Fatalf("unexpected staging branch config: %q (ok=%v)", staging, ok)
	}
}

func TestMergedPRNumbers(t *testing.T) {
	t.Parallel()

	workDir := setupRepositoryWithMergedPullRequests(t)
	git := NewGit(workDir)

	got, err := git.MergedPRNumbers(context.Background(), DefaultRemoteName, "master", "staging")
	if err != nil {
		t.Fatalf("merged pr numbers: %v", err)
	}

	want := []int{1}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
