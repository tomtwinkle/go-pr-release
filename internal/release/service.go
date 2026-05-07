package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
)

var ErrNoPullRequestsToRelease = errors.New("no pull requests to be released")

type Service struct {
	config Config
	git    *Git
	github GitHubClient
	stdout io.Writer
	stderr io.Writer
}

func NewService(config Config, stdout, stderr io.Writer) *Service {
	return NewServiceWithClients(config, NewGit(config.WorkDir), NewRESTGitHubClient(config), stdout, stderr)
}

func NewServiceWithClients(config Config, git *Git, github GitHubClient, stdout, stderr io.Writer) *Service {
	return &Service{
		config: config,
		git:    git,
		github: github,
		stdout: stdout,
		stderr: stderr,
	}
}

func (s *Service) Run(ctx context.Context) error {
	mergedPRs, err := s.fetchMergedPullRequests(ctx)
	if err != nil {
		return err
	}
	if len(mergedPRs) == 0 {
		s.say("No pull requests to be released")
		return ErrNoPullRequestsToRelease
	}

	root, err := s.git.Root(ctx)
	if err != nil {
		return err
	}

	existingPR, err := s.detectExistingReleasePullRequest(ctx)
	if err != nil {
		return err
	}

	createMode := existingPR == nil
	var changedFiles []ChangedFile
	switch {
	case createMode && s.config.DryRun:
		changedFiles = nil
	case createMode:
		existingPR, err = s.github.CreatePullRequest(
			ctx,
			"Preparing release pull request...",
			s.config.Repository.HeadRef(s.config.StagingBranch),
			s.config.ProductionBranch,
			"",
		)
		if err != nil {
			return err
		}
		changedFiles, err = s.github.ListPullRequestFiles(ctx, existingPR.Number)
		if err != nil {
			return err
		}
	default:
		changedFiles, err = s.github.ListPullRequestFiles(ctx, existingPR.Number)
		if err != nil {
			return err
		}
	}

	title, body, err := BuildTitleAndBody(
		root,
		existingPR,
		mergedPRs,
		changedFiles,
		s.config.TemplatePath,
		s.config.Mention,
	)
	if err != nil {
		return err
	}

	if s.config.Title != "" {
		title = s.config.Title
	}

	oldBody := ""
	if existingPR != nil {
		oldBody = existingPR.Body
	}
	if !s.config.OverwriteDescription {
		body = MergeBodies(oldBody, body)
	}

	if s.config.DryRun {
		s.say("Dry-run. Not updating PR")
		s.say(title)
		s.say(body)
		if s.config.JSON {
			s.dumpJSON(existingPR, mergedPRs, changedFiles)
		}
		return nil
	}

	releasePR, err := s.github.UpdatePullRequest(ctx, existingPR.Number, title, body)
	if err != nil {
		return err
	}

	if err := s.github.AddLabels(ctx, releasePR.Number, s.config.Labels); err != nil {
		return err
	}

	if s.config.AssignPRAuthor {
		assignees := collectMentionTargets(mergedPRs, s.config.Mention)
		if err := s.github.AddAssignees(ctx, releasePR.Number, assignees); err != nil {
			return err
		}
	}

	reviewers := append([]string(nil), s.config.ExtraReviewers...)
	if s.config.RequestPRAuthorReview {
		reviewers = append(reviewers, collectMentionTargets(mergedPRs, s.config.Mention)...)
	}
	reviewers = uniqueStrings(reviewers)
	if err := s.github.RequestReviewers(ctx, releasePR.Number, reviewers); err != nil {
		return err
	}

	mode := "Updated"
	if createMode {
		mode = "Created"
	}
	s.say(fmt.Sprintf("%s pull request: %s", mode, releasePR.URL))
	if s.config.JSON {
		s.dumpJSON(releasePR, mergedPRs, changedFiles)
	}

	return nil
}

func (s *Service) fetchMergedPullRequests(ctx context.Context) ([]PullRequest, error) {
	isShallow, err := s.git.IsShallow(ctx)
	if err != nil {
		return nil, err
	}
	if isShallow {
		if err := s.git.Unshallow(ctx); err != nil {
			return nil, err
		}
	}
	if !s.config.NoFetch {
		if err := s.git.RemoteUpdate(ctx, s.config.RemoteName); err != nil {
			return nil, err
		}
	}

	numbers, err := s.git.MergedPRNumbers(ctx, s.config.RemoteName, s.config.ProductionBranch, s.config.StagingBranch)
	if err != nil {
		return nil, err
	}
	if s.config.Squashed {
		squashNumbers, squashErr := s.fetchSquashMergedPullRequests(ctx)
		if squashErr != nil {
			return nil, squashErr
		}
		numbers = append(numbers, squashNumbers...)
	}

	numbers = uniqueInts(numbers)
	sort.Ints(numbers)

	pullRequests, err := s.github.GetPullRequests(ctx, numbers)
	if err != nil {
		return nil, err
	}

	mergedPullRequests := make([]PullRequest, 0, len(pullRequests))
	for _, pr := range pullRequests {
		if !pr.Merged {
			continue
		}
		mergedPullRequests = append(mergedPullRequests, pr)
	}

	return mergedPullRequests, nil
}

func (s *Service) fetchSquashMergedPullRequests(ctx context.Context) ([]int, error) {
	shas, err := s.git.SquashCommitSHAs(ctx, s.config.RemoteName, s.config.ProductionBranch, s.config.StagingBranch)
	if err != nil {
		return nil, err
	}
	var numbers []int
	for _, query := range buildSearchQueries(s.config.Repository.FullName(), shas) {
		found, err := s.github.SearchPullRequestNumbers(ctx, query)
		if err != nil {
			return nil, err
		}
		numbers = append(numbers, found...)
	}
	return numbers, nil
}

func (s *Service) detectExistingReleasePullRequest(ctx context.Context) (*PullRequest, error) {
	pullRequests, err := s.github.ListOpenReleasePullRequests(
		ctx,
		s.config.Repository.HeadRef(s.config.StagingBranch),
		s.config.ProductionBranch,
	)
	if err != nil {
		return nil, err
	}
	if len(pullRequests) == 0 {
		return nil, nil
	}
	return &pullRequests[0], nil
}

func (s *Service) dumpJSON(releasePR *PullRequest, mergedPRs []PullRequest, changedFiles []ChangedFile) {
	payload := struct {
		ReleasePullRequest *PullRequest  `json:"release_pull_request"`
		MergedPullRequests []PullRequest `json:"merged_pull_requests"`
		ChangedFiles       []ChangedFile `json:"changed_files"`
	}{
		ReleasePullRequest: releasePR,
		MergedPullRequests: mergedPRs,
		ChangedFiles:       changedFiles,
	}

	encoder := json.NewEncoder(s.stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func (s *Service) say(message string) {
	if strings.TrimSpace(message) == "" {
		return
	}
	fmt.Fprintln(s.stderr, message)
}

func collectMentionTargets(prs []PullRequest, mentionType string) []string {
	targets := make([]string, 0, len(prs))
	for _, pr := range prs {
		targets = append(targets, pr.TargetUserLoginNames(mentionType)...)
	}
	return uniqueStrings(targets)
}

func uniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func uniqueInts(values []int) []int {
	if len(values) == 0 {
		return nil
	}
	result := make([]int, 0, len(values))
	for _, value := range values {
		if slices.Contains(result, value) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func buildSearchQueries(repository string, shas []string) []string {
	if len(shas) == 0 {
		return nil
	}

	const maxQueryLength = 200
	baseQuery := fmt.Sprintf("repo:%s is:pr is:closed", repository)
	query := baseQuery
	queries := make([]string, 0, len(shas))

	for _, sha := range shas {
		if len(query)+1+len(sha) >= maxQueryLength {
			queries = append(queries, query)
			query = baseQuery
		}
		query += " " + sha
	}
	if query != baseQuery {
		queries = append(queries, query)
	}
	return queries
}
