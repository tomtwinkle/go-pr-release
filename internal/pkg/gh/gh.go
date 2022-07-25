package gh

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/storage/memory"

	"golang.org/x/sync/errgroup"

	"github.com/go-git/go-git/v5"

	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

const AsynchronousTimeout = 60 * time.Second

var (
	ErrBranchNotFound = errors.New("branch not found")
)

type Github interface {
	Config() *RemoteConfig
	CreateReleasePR(ctx context.Context, title, fromBranch, toBranch, body string) (*github.PullRequest, error)
	GetReleasePR(ctx context.Context, fromBranch, toBranch string) (*github.PullRequest, error)
	GetMergedPRs(ctx context.Context, fromBranch, toBranch string) (PullRequests, error)
	AssignReviews(ctx context.Context, prNumber int, reviewers ...string) (*github.PullRequest, error)
	Labeling(ctx context.Context, prNumber int, labels ...string) ([]*github.Label, error)
}

type gh struct {
	client *github.Client
	remote *git.Remote
	config *RemoteConfig
}

func New(ctx context.Context, token string, p RemoteConfigParam) (Github, error) {
	cnf, remote, err := gitRemoteConfig(p)
	if err != nil {
		return nil, err
	}
	return &gh{
		client: newClient(ctx, token),
		remote: remote,
		config: cnf,
	}, nil
}

func NewWithConfig(ctx context.Context, token string, remoteConfig RemoteConfig) Github {
	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: "origin",
		URLs: []string{fmt.Sprintf("https://github.com/%s/%s", remoteConfig.Owner, remoteConfig.Repo)},
	})
	return &gh{
		client: newClient(ctx, token),
		remote: remote,
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

func gitRemoteConfig(p RemoteConfigParam) (*RemoteConfig, *git.Remote, error) {
	r, err := git.PlainOpen(p.GitDirPath)
	if err != nil {
		return nil, nil, err
	}
	remote, err := r.Remote(p.RemoteName)
	if err != nil {
		return nil, nil, err
	}
	if len(remote.Config().URLs) == 0 {
		return nil, nil, errors.New("no set origin git urls")
	}
	url := remote.Config().URLs[0]
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "git@github.com:")
	url = strings.TrimSuffix(url, ".git")
	ss := strings.Split(url, "/")
	owner := ss[0]
	repo := ss[1]
	return &RemoteConfig{owner, repo}, remote, nil
}

type PullRequests []*github.PullRequest

func (prs PullRequests) FindHash(sha string) (*github.PullRequest, bool) {
	for _, v := range prs {
		v.Head.GetSHA()
		if v.GetMergeCommitSHA() == sha {
			return v, true
		}
	}
	return nil, false
}
func (prs PullRequests) SHAs() []string {
	shas := make([]string, len(prs))
	for i, v := range prs {
		shas[i] = v.GetMergeCommitSHA()
	}
	return shas
}

func (g *gh) Config() *RemoteConfig {
	return g.config
}

func (g *gh) BranchCompareCommits(ctx context.Context, fromBranch, toBranch string) ([]*github.RepositoryCommit, error) {
	const (
		MaxPages = 4   // 250 * 4 = 1000 commits
		PerPage  = 250 // Github API Limit: 250
	)
	getcommits := make([]*github.RepositoryCommit, 0, MaxPages*PerPage)
	for i := 0; i < MaxPages; i++ {
		page := i
		opts := &github.ListOptions{
			Page:    page,
			PerPage: PerPage,
		}
		commits, _, err := g.client.Repositories.CompareCommits(ctx, g.config.Owner, g.config.Repo, toBranch, fromBranch, opts)
		if err != nil {
			return nil, err
		}
		getcommits = append(getcommits, commits.Commits...)
		if commits.TotalCommits != nil && *commits.TotalCommits == len(getcommits) {
			break
		}
	}
	return getcommits, nil
}

func (g *gh) ClosedPRLists(ctx context.Context) (PullRequests, error) {
	const (
		MaxPages = 2   // 100 * 2 = 200 PullRequests
		PerPage  = 100 // Github API Limit: 100
	)
	var mu sync.Mutex
	eg, ctx := errgroup.WithContext(ctx)
	getprs := make([]*github.PullRequest, 0, MaxPages*PerPage)
	for i := 0; i < MaxPages; i++ {
		page := i
		eg.Go(func() error {
			opt := &github.PullRequestListOptions{
				State:     "closed",
				Sort:      "created",
				Direction: "desc",
				ListOptions: github.ListOptions{
					Page:    page,
					PerPage: PerPage,
				},
			}
			prs, _, err := g.client.PullRequests.List(ctx, g.config.Owner, g.config.Repo, opt)
			if err != nil {
				return err
			}
			mu.Lock()
			getprs = append(getprs, prs...)
			mu.Unlock()
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return getprs, nil
}

func (g *gh) GetMergedPRs(ctx context.Context, fromBranch, toBranch string) (PullRequests, error) {
	// Obtain a list of commit differences and pull requests that have already been closed
	var (
		commits []*github.RepositoryCommit
		allprs  PullRequests
	)
	beforectx, beforeCancel := context.WithTimeout(ctx, AsynchronousTimeout)
	defer beforeCancel()
	eg, beforectx := errgroup.WithContext(beforectx)
	eg.Go(func() error {
		var err error
		// Note: comparison limit 250 commits
		commits, err = g.BranchCompareCommits(beforectx, fromBranch, toBranch)
		if err != nil {
			return err
		}
		return nil
	})
	eg.Go(func() error {
		var err error
		allprs, err = g.ClosedPRLists(beforectx)
		if err != nil {
			return err
		}
		return nil
	})
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// commit hash is matched against the hash value in the PullRequests. If it cannot be obtained, get it from Github API
	listprs := make([]*github.PullRequest, 0, len(allprs))
	needAPICommitSHAs := make([]string, 0)
	for _, commit := range commits {
		var hitSHA bool
		sha := commit.GetSHA()
		if sha == "" {
			continue
		}
		if pr, ok := allprs.FindHash(sha); ok {
			listprs = append(listprs, pr)
			continue
		}
		for _, parent := range commit.Parents {
			if pr, ok := allprs.FindHash(parent.GetSHA()); ok {
				listprs = append(listprs, pr)
				hitSHA = true
				break
			}
		}
		if hitSHA {
			continue
		}
		needAPICommitSHAs = append(needAPICommitSHAs, sha)
	}

	afterctx, afterCancel := context.WithTimeout(ctx, AsynchronousTimeout)
	defer afterCancel()
	eg, afterctx = errgroup.WithContext(afterctx)
	var mu sync.Mutex
	for _, sha := range needAPICommitSHAs {
		eg.Go(func() error {
			prs, _, err := g.client.PullRequests.ListPullRequestsWithCommit(afterctx, g.config.Owner, g.config.Repo, sha, nil)
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
