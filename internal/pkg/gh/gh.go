package gh

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
	"golang.org/x/sync/errgroup"
)

const AsynchronousTimeout = 60 * time.Second

var (
	ErrBranchNotFound = errors.New("branch not found")
)

type Github interface {
	CreateReleasePR(ctx context.Context, title, fromBranch, toBranch, body string) (*github.PullRequest, error)
	GetReleasePR(ctx context.Context, fromBranch, toBranch string) (*github.PullRequest, error)
	GetMergedPRs(ctx context.Context, fromBranch, toBranch string) (PullRequests, error)
	AssignReviews(ctx context.Context, prNumber int, reviewers ...string) (*github.PullRequest, error)
	Labeling(ctx context.Context, prNumber int, labels ...string) ([]*github.Label, error)
}

type gh struct {
	client     *github.Client
	repository *git.Repository
	remote     *git.Remote
	config     *RemoteConfig

	logger *slog.Logger
}

func New(ctx context.Context, token string, p RemoteConfigParam) (Github, error) {
	cnf, repo, remote, err := gitRemoteConfig(p)
	if err != nil {
		return nil, err
	}
	logger := p.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &gh{
		client:     newClient(ctx, token),
		repository: repo,
		remote:     remote,
		config:     cnf,
		logger:     logger,
	}, nil
}

func NewWithConfig(ctx context.Context, token string, remoteConfig RemoteConfig) (Github, error) {
	r, err := git.CloneContext(ctx, memory.NewStorage(), nil, &git.CloneOptions{
		URL: fmt.Sprintf("https://github.com/%s/%s", remoteConfig.Owner, remoteConfig.Repo),
	})
	if err != nil {
		return nil, err
	}
	remote, err := r.Remote("origin")
	if err != nil {
		return nil, err
	}
	logger := remoteConfig.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &gh{
		client:     newClient(ctx, token),
		repository: r,
		remote:     remote,
		config:     &remoteConfig,
		logger:     logger,
	}, nil
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
	Logger     *slog.Logger
}

type RemoteConfig struct {
	Owner  string
	Repo   string
	Logger *slog.Logger
}

func gitRemoteConfig(p RemoteConfigParam) (*RemoteConfig, *git.Repository, *git.Remote, error) {
	r, err := git.PlainOpen(p.GitDirPath)
	if err != nil {
		return nil, nil, nil, err
	}
	remote, err := r.Remote(p.RemoteName)
	if err != nil {
		return nil, nil, nil, err
	}
	if len(remote.Config().URLs) == 0 {
		return nil, nil, nil, errors.New("no set origin git urls")
	}
	url := remote.Config().URLs[0]
	url = strings.TrimPrefix(url, "https://github.com/")
	url = strings.TrimPrefix(url, "git@github.com:")
	url = strings.TrimSuffix(url, ".git")
	ss := strings.Split(url, "/")
	owner := ss[0]
	repo := ss[1]
	return &RemoteConfig{Owner: owner, Repo: repo, Logger: p.Logger}, r, remote, nil
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

func (g *gh) GetMergedPRs(ctx context.Context, fromBranch, toBranch string) (PullRequests, error) {
	if err := g.remote.FetchContext(ctx, &git.FetchOptions{
		RemoteName: "origin",
	}); err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return nil, fmt.Errorf("git Remote Fetch error: %w", err)
		}
	}
	toHash, err := g.resolveBranch(ctx, toBranch)
	if err != nil {
		return nil, err
	}
	fromHash, err := g.resolveBranch(ctx, fromBranch)
	if err != nil {
		return nil, err
	}

	prNums, err := g.fetchMergedPRNumsFromGit(ctx, *fromHash, *toHash)
	if err != nil {
		return nil, err
	}
	prs, err := g.fetchPRsFromGithub(ctx, prNums)
	if err != nil {
		return nil, err
	}
	return prs, nil
}

func (g *gh) resolveBranch(ctx context.Context, remoteBranch string) (*plumbing.Hash, error) {
	revision := plumbing.Revision("refs/remotes/origin/" + remoteBranch)
	hash, err := g.repository.ResolveRevision(revision)
	if err != nil {
		return nil, fmt.Errorf("resolve error [%s]: %w", remoteBranch, err)
	}
	g.logger.DebugContext(ctx, fmt.Sprintf("resolve branch=%s, hash=%s", remoteBranch, hash.String()))
	return hash, nil
}

func (g *gh) fetchMergedPRNumsFromGit(ctx context.Context, fromHash, toHash plumbing.Hash) ([]int, error) {
	fromCommit, err := g.repository.CommitObject(fromHash)
	if err != nil {
		return nil, fmt.Errorf("github Git GetCommit fromHash error [%s]: %w", fromHash.String(), err)
	}
	toCommit, err := g.repository.CommitObject(toHash)
	if err != nil {
		return nil, fmt.Errorf("github Git GetCommit toHash error [%s]: %w", toHash.String(), err)
	}
	toCommitHashes := make(map[plumbing.Hash]struct{}, len(toCommit.ParentHashes))
	for _, v := range toCommit.ParentHashes {
		toCommitHashes[v] = struct{}{}
	}

	itr, err := g.repository.Log(&git.LogOptions{
		From:  fromHash,
		Order: git.LogOrderCommitterTime,
	})
	if err != nil {
		return nil, fmt.Errorf("git Repository Logs error [%s]: %w", fromHash, err)
	}

	var mergedFeatureHeadCommits = make(map[plumbing.Hash]*object.Commit)
	if err := itr.ForEach(func(c *object.Commit) error {
		if _, ok := toCommitHashes[c.Hash]; ok {
			return storer.ErrStop
		}
		mergeCommits, err := fromCommit.MergeBase(c)
		if err != nil {
			return fmt.Errorf("git MergeBase error: %w", err)
		}
		for _, mc := range mergeCommits {
			g.logger.DebugContext(ctx, fmt.Sprintf("git repo log hash=%s commit=%s", c.Hash.String(), c.Message))
			mergedFeatureHeadCommits[mc.Hash] = mc
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("git Repository Logs Iterate error: %w", err)
	}

	refs, err := g.remote.List(&git.ListOptions{
		// Returns all references, including peeled references.
		PeelingOption: git.AppendPeeled,
	})
	if err != nil {
		return nil, fmt.Errorf("git Remote refs List error: %w", err)
	}

	var prNums = make([]int, 0, len(refs))
	for _, ref := range refs {
		if _, ok := mergedFeatureHeadCommits[ref.Hash()]; !ok {
			continue
		}
		refName := ref.Name().String()
		if RegRefPullRequest.MatchString(refName) {
			prNum, err := strconv.Atoi(RegRefPullRequest.ReplaceAllString(refName, "$1"))
			if err != nil {
				return nil, err
			}
			prNums = append(prNums, prNum)
		}
	}
	return prNums, nil
}

func (g *gh) fetchPRsFromGithub(ctx context.Context, prNums []int) (PullRequests, error) {
	var (
		eg       errgroup.Group
		fetchPrs = make(PullRequests, len(prNums))
	)
	for i, prNum := range prNums {
		i := i
		prNum := prNum
		eg.Go(func() error {
			pr, _, err := g.client.PullRequests.Get(ctx, g.config.Owner, g.config.Repo, prNum)
			if err != nil {
				return fmt.Errorf("github PullRequest Get error [%d]: %w", prNum, err)
			}
			fetchPrs[i] = pr
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	prs := make(PullRequests, 0, len(prNums))
	for _, pr := range fetchPrs {
		if pr == nil {
			continue
		}
		if pr.Merged != nil && *pr.Merged {
			prs = append(prs, pr)
		}
	}
	return prs, nil
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
		return nil, fmt.Errorf("github PullRequest List error head=%s, base=%s: %w", opt.Head, toBranch, err)
	}
	for _, pr := range prs {
		if pr.GetBase().GetRef() == toBranch && pr.GetHead().GetRef() == fromBranch && pr.MergedAt == nil {
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
