package release

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestBuildSearchQueries(t *testing.T) {
	t.Parallel()

	shas := []string{
		"abcdef0", "abcdef1", "abcdef2", "abcdef3", "abcdef4",
		"abcdef5", "abcdef6", "abcdef7", "abcdef8", "abcdef9",
		"bcdefg0", "bcdefg1", "bcdefg2", "bcdefg3", "bcdefg4",
		"bcdefg5", "bcdefg6", "bcdefg7", "bcdefg8", "bcdefg9",
		"cdefgh0", "cdefgh1", "cdefgh2", "cdefgh3", "cdefgh4",
		"cdefgh5", "cdefgh6", "cdefgh7", "cdefgh8", "cdefgh9",
	}
	queries := buildSearchQueries("octo/example", shas)
	if len(queries) < 2 {
		t.Fatalf("expected multiple queries, got %d", len(queries))
	}
	for _, query := range queries {
		if len(query) >= 200 {
			t.Fatalf("query too long: %d", len(query))
		}
	}
}

func TestServiceFetchMergedPullRequestsIncludesAllChainedPullRequests(t *testing.T) {
	t.Parallel()

	workDir := setupRepositoryWithChainedAndUnrelatedPullRequests(t)
	fakeGitHub := &fakeGitHubClient{
		pullRequests: map[int]PullRequest{
			1: {Number: 1, Title: "feature1", Merged: true, User: User{LoginName: "alice"}},
			2: {Number: 2, Title: "feature2", Merged: true, User: User{LoginName: "bob"}},
			3: {Number: 3, Title: "feature3", Merged: true, User: User{LoginName: "carol"}},
			4: {Number: 4, Title: "feature4", Merged: true, User: User{LoginName: "dave"}},
		},
	}

	service := NewServiceWithClients(Config{
		WorkDir:          workDir,
		RemoteName:       DefaultRemoteName,
		Repository:       Repository{Owner: "octo", Name: "example", Scheme: "https"},
		Token:            "dummy",
		ProductionBranch: "master",
		StagingBranch:    "staging",
	}, NewGit(workDir), fakeGitHub, &bytes.Buffer{}, &bytes.Buffer{})

	pullRequests, err := service.fetchMergedPullRequests(context.Background())
	if err != nil {
		t.Fatalf("fetch merged pull requests: %v", err)
	}

	if got, want := pullRequestNumbers(pullRequests), []int{1, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestServiceFetchMergedPullRequestsExcludesUnrelatedPullRequests(t *testing.T) {
	t.Parallel()

	workDir := setupRepositoryWithChainedAndUnrelatedPullRequests(t)
	fakeGitHub := &fakeGitHubClient{
		pullRequests: map[int]PullRequest{
			1: {Number: 1, Title: "feature1", Merged: true, User: User{LoginName: "alice"}},
			2: {Number: 2, Title: "feature2", Merged: true, User: User{LoginName: "bob"}},
			3: {Number: 3, Title: "feature3", Merged: true, User: User{LoginName: "carol"}},
			4: {Number: 4, Title: "feature4", Merged: true, User: User{LoginName: "dave"}},
		},
	}

	service := NewServiceWithClients(Config{
		WorkDir:          workDir,
		RemoteName:       DefaultRemoteName,
		Repository:       Repository{Owner: "octo", Name: "example", Scheme: "https"},
		Token:            "dummy",
		ProductionBranch: "master",
		StagingBranch:    "staging",
	}, NewGit(workDir), fakeGitHub, &bytes.Buffer{}, &bytes.Buffer{})

	pullRequests, err := service.fetchMergedPullRequests(context.Background())
	if err != nil {
		t.Fatalf("fetch merged pull requests: %v", err)
	}

	got := pullRequestNumbers(pullRequests)
	if slicesContains(got, 3) || slicesContains(got, 4) {
		t.Fatalf("unexpected unrelated pull requests found: %v", got)
	}
}

func TestServiceRunDryRun(t *testing.T) {
	t.Parallel()

	workDir := setupRepositoryWithMergedPullRequests(t)

	fakeGitHub := &fakeGitHubClient{
		pullRequests: map[int]PullRequest{
			1: {
				Number: 1,
				Title:  "Add feature",
				Merged: true,
				User:   User{LoginName: "alice"},
			},
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	service := NewServiceWithClients(Config{
		WorkDir:          workDir,
		RemoteName:       DefaultRemoteName,
		Repository:       Repository{Owner: "octo", Name: "example", Scheme: "https"},
		Token:            "dummy",
		ProductionBranch: "master",
		StagingBranch:    "staging",
		Title:            "Custom release",
		DryRun:           true,
	}, NewGit(workDir), fakeGitHub, &stdout, &stderr)

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("service run: %v", err)
	}

	if fakeGitHub.updateCalled {
		t.Fatalf("expected no update call on dry-run")
	}
	if !strings.Contains(stderr.String(), "Dry-run. Not updating PR") {
		t.Fatalf("stderr does not contain dry-run message: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "Custom release") {
		t.Fatalf("stderr does not contain title: %q", stderr.String())
	}
	if !strings.Contains(stderr.String(), "- [ ] #1 @alice") {
		t.Fatalf("stderr does not contain checklist item: %q", stderr.String())
	}
}

func TestServiceRunUpdatesExistingReleasePullRequest(t *testing.T) {
	t.Parallel()

	workDir := setupRepositoryWithMergedPullRequests(t)

	fakeGitHub := &fakeGitHubClient{
		pullRequests: map[int]PullRequest{
			1: {
				Number: 1,
				Title:  "Add feature",
				Merged: true,
				User:   User{LoginName: "alice"},
			},
		},
		releasePullRequests: []PullRequest{
			{
				Number: 99,
				Body:   "- [x] #1 @alice",
				URL:    "https://example.com/pulls/99",
			},
		},
		changedFiles: map[int][]ChangedFile{
			99: []ChangedFile{{Filename: "README.md"}},
		},
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	service := NewServiceWithClients(Config{
		WorkDir:               workDir,
		RemoteName:            DefaultRemoteName,
		Repository:            Repository{Owner: "octo", Name: "example", Scheme: "https"},
		Token:                 "dummy",
		ProductionBranch:      "master",
		StagingBranch:         "staging",
		Title:                 "Release title",
		Labels:                []string{"release"},
		ExtraReviewers:        []string{"bob"},
		AssignPRAuthor:        true,
		RequestPRAuthorReview: true,
	}, NewGit(workDir), fakeGitHub, &stdout, &stderr)

	if err := service.Run(context.Background()); err != nil {
		t.Fatalf("service run: %v", err)
	}

	if !fakeGitHub.updateCalled {
		t.Fatalf("expected update call")
	}
	if fakeGitHub.updatedTitle != "Release title" {
		t.Fatalf("unexpected updated title: %q", fakeGitHub.updatedTitle)
	}
	if !strings.Contains(fakeGitHub.updatedBody, "- [x] #1 @alice") {
		t.Fatalf("expected merged checklist status in body: %q", fakeGitHub.updatedBody)
	}
	if !reflect.DeepEqual(fakeGitHub.labels, []string{"release"}) {
		t.Fatalf("unexpected labels: %v", fakeGitHub.labels)
	}
	if !reflect.DeepEqual(fakeGitHub.assignees, []string{"alice"}) {
		t.Fatalf("unexpected assignees: %v", fakeGitHub.assignees)
	}
	if !reflect.DeepEqual(fakeGitHub.reviewers, []string{"bob", "alice"}) {
		t.Fatalf("unexpected reviewers: %v", fakeGitHub.reviewers)
	}
	if !strings.Contains(stderr.String(), "Updated pull request: https://example.com/pulls/99") {
		t.Fatalf("stderr does not contain update message: %q", stderr.String())
	}
}

type fakeGitHubClient struct {
	pullRequests        map[int]PullRequest
	releasePullRequests []PullRequest
	changedFiles        map[int][]ChangedFile

	updateCalled  bool
	updatedTitle  string
	updatedBody   string
	labels        []string
	assignees     []string
	reviewers     []string
	searchQueries []string
}

func (f *fakeGitHubClient) GetPullRequests(_ context.Context, numbers []int) ([]PullRequest, error) {
	pullRequests := make([]PullRequest, 0, len(numbers))
	for _, number := range numbers {
		pr, ok := f.pullRequests[number]
		if !ok {
			continue
		}
		pullRequests = append(pullRequests, pr)
	}
	return pullRequests, nil
}

func (f *fakeGitHubClient) ListOpenReleasePullRequests(_ context.Context, head, base string) ([]PullRequest, error) {
	return f.releasePullRequests, nil
}

func (f *fakeGitHubClient) CreatePullRequest(_ context.Context, title, head, base, body string) (*PullRequest, error) {
	pr := PullRequest{Number: 100, Title: title, Body: body, URL: "https://example.com/pulls/100"}
	return &pr, nil
}

func (f *fakeGitHubClient) UpdatePullRequest(_ context.Context, number int, title, body string) (*PullRequest, error) {
	f.updateCalled = true
	f.updatedTitle = title
	f.updatedBody = body
	pr := PullRequest{Number: number, Title: title, Body: body, URL: "https://example.com/pulls/99"}
	return &pr, nil
}

func (f *fakeGitHubClient) AddLabels(_ context.Context, number int, labels []string) error {
	f.labels = append([]string(nil), labels...)
	return nil
}

func (f *fakeGitHubClient) AddAssignees(_ context.Context, number int, assignees []string) error {
	f.assignees = append([]string(nil), assignees...)
	return nil
}

func (f *fakeGitHubClient) RequestReviewers(_ context.Context, number int, reviewers []string) error {
	f.reviewers = append([]string(nil), reviewers...)
	return nil
}

func (f *fakeGitHubClient) ListPullRequestFiles(_ context.Context, number int) ([]ChangedFile, error) {
	return f.changedFiles[number], nil
}

func (f *fakeGitHubClient) SearchPullRequestNumbers(_ context.Context, query string) ([]int, error) {
	f.searchQueries = append(f.searchQueries, query)
	return nil, nil
}

func slicesContains(values []int, target int) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
