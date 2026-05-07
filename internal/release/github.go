package release

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GitHubClient interface {
	GetPullRequests(ctx context.Context, numbers []int) ([]PullRequest, error)
	ListOpenReleasePullRequests(ctx context.Context, head, base string) ([]PullRequest, error)
	CreatePullRequest(ctx context.Context, title, head, base, body string) (*PullRequest, error)
	UpdatePullRequest(ctx context.Context, number int, title, body string) (*PullRequest, error)
	AddLabels(ctx context.Context, number int, labels []string) error
	AddAssignees(ctx context.Context, number int, assignees []string) error
	RequestReviewers(ctx context.Context, number int, reviewers []string) error
	ListPullRequestFiles(ctx context.Context, number int) ([]ChangedFile, error)
	SearchPullRequestNumbers(ctx context.Context, query string) ([]int, error)
}

type RESTGitHubClient struct {
	httpClient *http.Client
	baseURL    string
	repository Repository
	token      string
}

func NewRESTGitHubClient(config Config) *RESTGitHubClient {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if config.InsecureSkipTLSVerify {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec
	}

	return &RESTGitHubClient{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		baseURL:    config.Repository.APIBaseURL(),
		repository: config.Repository,
		token:      config.Token,
	}
}

func (c *RESTGitHubClient) GetPullRequests(ctx context.Context, numbers []int) ([]PullRequest, error) {
	numbers = uniqueInts(numbers)
	if len(numbers) == 0 {
		return nil, nil
	}

	remaining := make(map[int]struct{}, len(numbers))
	for _, number := range numbers {
		remaining[number] = struct{}{}
	}

	found := make(map[int]PullRequest, len(numbers))

	const pageSize = 100
	for page := 1; len(remaining) > 0; page++ {
		response, err := c.listClosedPullRequestsPage(ctx, page, pageSize)
		if err != nil {
			return nil, err
		}
		if len(response) == 0 {
			break
		}

		for _, pr := range response {
			if _, ok := remaining[pr.Number]; !ok {
				continue
			}
			found[pr.Number] = pr.toDomain()
			delete(remaining, pr.Number)
		}

		if len(response) < pageSize {
			break
		}
	}

	if len(remaining) > 0 {
		for _, number := range numbers {
			if _, ok := remaining[number]; !ok {
				continue
			}
			pr, err := c.getPullRequest(ctx, number)
			if err != nil {
				return nil, err
			}
			found[number] = *pr
		}
	}

	pullRequests := make([]PullRequest, 0, len(found))
	for _, number := range numbers {
		if pr, ok := found[number]; ok {
			pullRequests = append(pullRequests, pr)
		}
	}

	return pullRequests, nil
}

func (c *RESTGitHubClient) getPullRequest(ctx context.Context, number int) (*PullRequest, error) {
	var response pullRequestDTO
	if err := c.request(
		ctx,
		http.MethodGet,
		fmt.Sprintf("repos/%s/pulls/%d", c.repository.FullName(), number),
		nil,
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	pr := response.toDomain()
	return &pr, nil
}

func (c *RESTGitHubClient) listClosedPullRequestsPage(ctx context.Context, page, pageSize int) ([]pullRequestDTO, error) {
	query := url.Values{}
	query.Set("state", "closed")
	query.Set("sort", "created")
	query.Set("direction", "desc")
	query.Set("per_page", fmt.Sprintf("%d", pageSize))
	query.Set("page", fmt.Sprintf("%d", page))

	var response []pullRequestDTO
	if err := c.request(
		ctx,
		http.MethodGet,
		fmt.Sprintf("repos/%s/pulls", c.repository.FullName()),
		query,
		nil,
		&response,
	); err != nil {
		return nil, err
	}

	return response, nil
}

func (c *RESTGitHubClient) ListOpenReleasePullRequests(ctx context.Context, head, base string) ([]PullRequest, error) {
	query := url.Values{}
	query.Set("state", "open")
	query.Set("head", head)
	query.Set("base", base)

	var response []pullRequestDTO
	if err := c.request(
		ctx,
		http.MethodGet,
		fmt.Sprintf("repos/%s/pulls", c.repository.FullName()),
		query,
		nil,
		&response,
	); err != nil {
		return nil, err
	}

	pullRequests := make([]PullRequest, 0, len(response))
	for _, pr := range response {
		pullRequests = append(pullRequests, pr.toDomain())
	}
	return pullRequests, nil
}

func (c *RESTGitHubClient) CreatePullRequest(ctx context.Context, title, head, base, body string) (*PullRequest, error) {
	request := map[string]string{
		"title": title,
		"head":  head,
		"base":  base,
		"body":  body,
	}

	var response pullRequestDTO
	if err := c.request(
		ctx,
		http.MethodPost,
		fmt.Sprintf("repos/%s/pulls", c.repository.FullName()),
		nil,
		request,
		&response,
	); err != nil {
		return nil, err
	}

	pr := response.toDomain()
	return &pr, nil
}

func (c *RESTGitHubClient) UpdatePullRequest(ctx context.Context, number int, title, body string) (*PullRequest, error) {
	request := map[string]string{
		"title": title,
		"body":  body,
	}

	var response pullRequestDTO
	if err := c.request(
		ctx,
		http.MethodPatch,
		fmt.Sprintf("repos/%s/pulls/%d", c.repository.FullName(), number),
		nil,
		request,
		&response,
	); err != nil {
		return nil, err
	}

	pr := response.toDomain()
	return &pr, nil
}

func (c *RESTGitHubClient) AddLabels(ctx context.Context, number int, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	request := map[string][]string{"labels": labels}
	return c.request(
		ctx,
		http.MethodPost,
		fmt.Sprintf("repos/%s/issues/%d/labels", c.repository.FullName(), number),
		nil,
		request,
		nil,
	)
}

func (c *RESTGitHubClient) AddAssignees(ctx context.Context, number int, assignees []string) error {
	if len(assignees) == 0 {
		return nil
	}
	request := map[string][]string{"assignees": assignees}
	return c.request(
		ctx,
		http.MethodPost,
		fmt.Sprintf("repos/%s/issues/%d/assignees", c.repository.FullName(), number),
		nil,
		request,
		nil,
	)
}

func (c *RESTGitHubClient) RequestReviewers(ctx context.Context, number int, reviewers []string) error {
	if len(reviewers) == 0 {
		return nil
	}
	request := map[string][]string{"reviewers": reviewers}
	return c.request(
		ctx,
		http.MethodPost,
		fmt.Sprintf("repos/%s/pulls/%d/requested_reviewers", c.repository.FullName(), number),
		nil,
		request,
		nil,
	)
}

func (c *RESTGitHubClient) ListPullRequestFiles(ctx context.Context, number int) ([]ChangedFile, error) {
	const pageSize = 100

	var files []ChangedFile
	for page := 1; ; page++ {
		query := url.Values{}
		query.Set("per_page", fmt.Sprintf("%d", pageSize))
		query.Set("page", fmt.Sprintf("%d", page))

		var response []changedFileDTO
		if err := c.request(
			ctx,
			http.MethodGet,
			fmt.Sprintf("repos/%s/pulls/%d/files", c.repository.FullName(), number),
			query,
			nil,
			&response,
		); err != nil {
			return nil, err
		}

		for _, file := range response {
			files = append(files, file.toDomain())
		}

		if len(response) < pageSize {
			break
		}
	}

	return files, nil
}

func (c *RESTGitHubClient) SearchPullRequestNumbers(ctx context.Context, query string) ([]int, error) {
	params := url.Values{}
	params.Set("q", query)

	var response searchIssuesResponse
	if err := c.request(ctx, http.MethodGet, "search/issues", params, nil, &response); err != nil {
		return nil, err
	}

	numbers := make([]int, 0, len(response.Items))
	for _, item := range response.Items {
		numbers = append(numbers, item.Number)
	}
	return numbers, nil
}

func (c *RESTGitHubClient) request(
	ctx context.Context,
	method string,
	path string,
	query url.Values,
	requestBody any,
	responseBody any,
) error {
	endpoint, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("parse github base url: %w", err)
	}
	endpoint.Path = strings.TrimSuffix(endpoint.Path, "/") + "/" + strings.TrimPrefix(path, "/")
	endpoint.RawQuery = query.Encode()

	var body io.Reader
	if requestBody != nil {
		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(requestBody); err != nil {
			return fmt.Errorf("encode github request: %w", err)
		}
		body = &buf
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return fmt.Errorf("create github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "go-pr-release")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, endpoint.Path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("%s %s: unexpected status %d: %s", method, endpoint.Path, resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	if responseBody == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
		return fmt.Errorf("decode github response: %w", err)
	}
	return nil
}

type searchIssuesResponse struct {
	Items []struct {
		Number int `json:"number"`
	} `json:"items"`
}

type userDTO struct {
	Login     string `json:"login"`
	HTMLURL   string `json:"html_url"`
	AvatarURL string `json:"avatar_url"`
}

func (u userDTO) toDomain() User {
	return User{
		LoginName: u.Login,
		URL:       u.HTMLURL,
		Avatar:    u.AvatarURL,
	}
}

type pullRequestDTO struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	HTMLURL        string     `json:"html_url"`
	State          string     `json:"state"`
	Merged         bool       `json:"merged"`
	MergeCommitSHA string     `json:"merge_commit_sha"`
	MergedAt       *time.Time `json:"merged_at"`
	User           userDTO    `json:"user"`
	Assignee       *userDTO   `json:"assignee"`
	Assignees      []userDTO  `json:"assignees"`
	Head           struct {
		Ref string `json:"ref"`
	} `json:"head"`
	Base struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

func (pr pullRequestDTO) toDomain() PullRequest {
	domain := PullRequest{
		Number:         pr.Number,
		Title:          pr.Title,
		Body:           pr.Body,
		URL:            pr.HTMLURL,
		State:          pr.State,
		Merged:         pr.Merged,
		MergeCommitSHA: pr.MergeCommitSHA,
		HeadRef:        pr.Head.Ref,
		BaseRef:        pr.Base.Ref,
		User:           pr.User.toDomain(),
		Assignees:      make([]User, 0, len(pr.Assignees)),
	}
	if pr.MergedAt != nil {
		domain.MergedAt = *pr.MergedAt
	}
	if pr.Assignee != nil {
		assignee := pr.Assignee.toDomain()
		domain.Assignee = &assignee
	}
	for _, assignee := range pr.Assignees {
		domain.Assignees = append(domain.Assignees, assignee.toDomain())
	}
	return domain
}

type changedFileDTO struct {
	Filename    string `json:"filename"`
	Status      string `json:"status"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	Changes     int    `json:"changes"`
	BlobURL     string `json:"blob_url"`
	RawURL      string `json:"raw_url"`
	ContentsURL string `json:"contents_url"`
	Patch       string `json:"patch"`
}

func (f changedFileDTO) toDomain() ChangedFile {
	return ChangedFile{
		Filename:    f.Filename,
		Status:      f.Status,
		Additions:   f.Additions,
		Deletions:   f.Deletions,
		Changes:     f.Changes,
		BlobURL:     f.BlobURL,
		RawURL:      f.RawURL,
		ContentsURL: f.ContentsURL,
		Patch:       f.Patch,
	}
}
