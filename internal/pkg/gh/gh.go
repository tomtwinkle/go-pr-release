package gh

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-git/go-git/v5"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

var (
	ErrBranchNotFound = errors.New("branch not found")
)

type Github interface {
	CreateReleasePR(ctx context.Context, owner, repo, title, fromBranch, toBranch, body string) (*github.PullRequest, error)
	GetMergedPRs(ctx context.Context, owner, repo, fromBranch, toBranch string) ([]*github.PullRequest, error)
}

type gh struct {
	client *github.Client
}

func New(ctx context.Context, token string) Github {
	return &gh{
		client: newClient(ctx, token),
	}
}

func newClient(ctx context.Context, token string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc)
}

type RemoteConfig struct {
	Owner string
	Repo  string
}

func GitRemoteConfig(path, name string) (*RemoteConfig, error) {
	r, err := git.PlainOpen(path)
	if err != nil {
		return nil, err
	}
	remote, err := r.Remote(name)
	if err != nil {
		return nil, err
	}
	if len(remote.Config().URLs) == 0 {
		return nil, errors.New("no set origin git urls")
	}
	url := remote.Config().URLs[0]
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "git@github.com:")
	reg := regexp.MustCompile(`^([^/]+)/(.+)\.git$`)

	owner := reg.ReplaceAllString(url, "$1")
	repo := reg.ReplaceAllString(url, "$2")

	return &RemoteConfig{owner, repo}, nil
}

func (g *gh) GetMergedPRs(ctx context.Context, owner, repo, fromBranch, toBranch string) ([]*github.PullRequest, error) {
	base, _, err := g.client.Repositories.GetBranch(ctx, owner, repo, toBranch, true)
	if err != nil {
		return nil, err
	}
	var lastMergedSHA *string
	if cms := len(base.GetCommit().Parents); cms > 0 {
		lastMergedSHA = base.GetCommit().Parents[cms-1].SHA
	}
	opt := &github.PullRequestListOptions{
		State:       "closed",
		Head:        fmt.Sprintf("origin/%s", toBranch),
		Base:        fromBranch,
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{},
	}
	prs, _, err := g.client.PullRequests.List(ctx, owner, repo, opt)
	if err != nil {
		return nil, err
	}
	mergedPRs := make([]*github.PullRequest, 0, len(prs))
	for _, pr := range prs {
		if lastMergedSHA != nil && *lastMergedSHA == pr.GetMergeCommitSHA() {
			break
		}
		if pr.MergedAt != nil {
			mergedPRs = append(mergedPRs, pr)
		}
	}
	return mergedPRs, nil
}

func (g *gh) GetReleasePR(ctx context.Context, owner, repo, fromBranch, toBranch string) (*github.PullRequest, error) {
	base, _, err := g.client.Repositories.GetBranch(ctx, owner, repo, fromBranch, true)
	if err != nil {
		return nil, err
	}
	var existsPR *github.PullRequest
	if baseCommit := base.GetCommit(); baseCommit != nil {
		opt := &github.PullRequestListOptions{
			State:       "open",
			Head:        fmt.Sprintf("origin/%s", toBranch),
			Base:        fromBranch,
			Sort:        "created",
			Direction:   "desc",
			ListOptions: github.ListOptions{},
		}
		prs, _, err := g.client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repo, baseCommit.GetSHA(), opt)
		if err != nil {
			return nil, err
		}
		for _, pr := range prs {
			if baseCommit.GetSHA() == pr.GetHead().GetSHA() {
				if pr.MergedAt == nil {
					existsPR = pr
					break
				}
			}
		}
	}
	if existsPR == nil {
		return nil, ErrBranchNotFound
	}
	return existsPR, nil
}

func (g *gh) CreateReleasePR(ctx context.Context, owner, repo, title, fromBranch, toBranch, body string) (*github.PullRequest, error) {
	var basePR *github.PullRequest
	if pr, err := g.GetReleasePR(ctx, owner, repo, fromBranch, toBranch); err != nil {
		if !errors.Is(ErrBranchNotFound, err) {
			return nil, err
		}
	} else {
		basePR = pr
	}

	if basePR != nil {
		basePR.Title = github.String(title)
		basePR.Body = github.String(body)
		pr, _, err := g.client.PullRequests.Edit(ctx, owner, repo, basePR.GetNumber(), basePR)
		if err != nil {
			return nil, err
		}
		return pr, nil
	} else {
		newPR := &github.NewPullRequest{
			Title: github.String(title),
			Head:  github.String(fmt.Sprintf("%s:%s", owner, fromBranch)),
			Base:  github.String(toBranch),
			Body:  github.String(body),
		}
		pr, _, err := g.client.PullRequests.Create(ctx, owner, repo, newPR)
		if err != nil {
			return nil, err
		}
		return pr, nil
	}
}
