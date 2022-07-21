package gh

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/go-git/go-git/v5"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

var (
	ErrBranchNotFound = errors.New("branch not found")
)

type Github interface {
	Config() *RemoteConfig
	CreateReleasePR(ctx context.Context, title, fromBranch, toBranch, body string) (*github.PullRequest, error)
	GetReleasePR(ctx context.Context, fromBranch, toBranch string) (*github.PullRequest, error)
	GetMergedPRs(ctx context.Context, fromBranch, toBranch string) ([]*github.PullRequest, error)
	AssignReviews(ctx context.Context, prNumber int, reviewers ...string) (*github.PullRequest, error)
	Labeling(ctx context.Context, prNumber int, labels ...string) ([]*github.Label, error)
}

type gh struct {
	client *github.Client
	config *RemoteConfig
}

func New(ctx context.Context, token string, p RemoteConfigParam) (Github, error) {
	config, err := gitRemoteConfig(p)
	if err != nil {
		return nil, err
	}
	return &gh{
		client: newClient(ctx, token),
		config: config,
	}, nil
}

func NewWithConfig(ctx context.Context, token string, remoteConfig RemoteConfig) Github {
	return &gh{
		client: newClient(ctx, token),
		config: &remoteConfig,
	}
}

func newClient(ctx context.Context, token string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	return github.NewClient(tc)
}

type RemoteConfigParam struct {
	GitDirPath string
	RemoteName string
}

type RemoteConfig struct {
	Owner string
	Repo  string
}

func gitRemoteConfig(p RemoteConfigParam) (*RemoteConfig, error) {
	r, err := git.PlainOpen(p.GitDirPath)
	if err != nil {
		return nil, err
	}
	remote, err := r.Remote(p.RemoteName)
	if err != nil {
		return nil, err
	}
	if len(remote.Config().URLs) == 0 {
		return nil, errors.New("no set origin git urls")
	}
	url := remote.Config().URLs[0]
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "git@github.com:")
	url = strings.TrimSuffix(url, ".git")
	ss := strings.Split(url, "/")
	owner := ss[0]
	repo := ss[1]
	return &RemoteConfig{owner, repo}, nil
}

func (g *gh) Config() *RemoteConfig {
	return g.config
}

func (g *gh) GetMergedPRs(ctx context.Context, fromBranch, toBranch string) ([]*github.PullRequest, error) {
	commits, _, err := g.client.Repositories.CompareCommits(ctx, g.config.Owner, g.config.Repo, toBranch, fromBranch, nil)
	if err != nil {
		return nil, err
	}
	var mu sync.Mutex
	eg, ctx := errgroup.WithContext(ctx)
	listprs := make([]*github.PullRequest, 0)
	for _, commit := range commits.Commits {
		sha := commit.GetSHA()
		eg.Go(func() error {
			prs, _, err := g.client.PullRequests.ListPullRequestsWithCommit(ctx, g.config.Owner, g.config.Repo, sha, nil)
			if err != nil {
				return err
			}
			for _, pr := range prs {
				if pr.MergedAt != nil {
					mu.Lock()
					listprs = append(listprs, pr)
					mu.Unlock()
				}
			}
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	sort.Slice(listprs, func(i, j int) bool {
		return listprs[i].GetNumber() > listprs[j].GetNumber()
	})

	mergedPRs := make([]*github.PullRequest, 0, len(listprs))
	uniq := make(map[int]struct{})
	for _, v := range listprs {
		if _, ok := uniq[v.GetNumber()]; ok {
			continue
		}
		uniq[v.GetNumber()] = struct{}{}
		mergedPRs = append(mergedPRs, v)
	}
	return mergedPRs, nil
}

func (g *gh) GetReleasePR(ctx context.Context, fromBranch, toBranch string) (*github.PullRequest, error) {
	var existsPR *github.PullRequest
	opt := &github.PullRequestListOptions{
		State:       "open",
		Head:        fmt.Sprintf("origin/%s", fromBranch),
		Base:        toBranch,
		Sort:        "created",
		Direction:   "desc",
		ListOptions: github.ListOptions{},
	}
	prs, _, err := g.client.PullRequests.List(ctx, g.config.Owner, g.config.Repo, opt)
	if err != nil {
		return nil, err
	}
	for _, pr := range prs {
		if pr.MergedAt == nil {
			existsPR = pr
			break
		}
	}
	if existsPR == nil {
		return nil, ErrBranchNotFound
	}
	return existsPR, nil
}

func (g *gh) CreateReleasePR(ctx context.Context, title, fromBranch, toBranch, body string) (*github.PullRequest, error) {
	var basePR *github.PullRequest
	if pr, err := g.GetReleasePR(ctx, fromBranch, toBranch); err != nil {
		if !errors.Is(ErrBranchNotFound, err) {
			return nil, err
		}
	} else {
		basePR = pr
	}

	if basePR != nil {
		basePR.Title = github.String(title)
		basePR.Body = github.String(body)
		pr, _, err := g.client.PullRequests.Edit(ctx, g.config.Owner, g.config.Repo, basePR.GetNumber(), basePR)
		if err != nil {
			return nil, err
		}
		return pr, nil
	} else {
		newPR := &github.NewPullRequest{
			Title: github.String(title),
			Head:  github.String(fmt.Sprintf("%s:%s", g.config.Owner, fromBranch)),
			Base:  github.String(toBranch),
			Body:  github.String(body),
		}
		pr, _, err := g.client.PullRequests.Create(ctx, g.config.Owner, g.config.Repo, newPR)
		if err != nil {
			return nil, err
		}
		return pr, nil
	}
}

func (g *gh) AssignReviews(ctx context.Context, prNumber int, reviewers ...string) (*github.PullRequest, error) {
	pr, _, err := g.client.PullRequests.RequestReviewers(ctx, g.config.Owner, g.config.Repo, prNumber, github.ReviewersRequest{
		Reviewers: reviewers,
	})
	if err != nil {
		return nil, err
	}
	return pr, nil
}

func (g *gh) Labeling(ctx context.Context, prNumber int, labels ...string) ([]*github.Label, error) {
	resLabels, _, err := g.client.Issues.AddLabelsToIssue(ctx, g.config.Owner, g.config.Repo, prNumber, labels)
	if err != nil {
		return nil, err
	}
	return resLabels, nil
}
