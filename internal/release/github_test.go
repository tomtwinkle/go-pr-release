package release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestRESTGitHubClientGetPullRequestsPaginatesBeyondOneHundredPullRequests(t *testing.T) {
	t.Parallel()

	client, pageRequests, detailRequests := newPaginatedPullRequestClient(t, 101)
	numbers := sequence(1, 101)

	pullRequests, err := client.GetPullRequests(context.Background(), numbers)
	if err != nil {
		t.Fatalf("get pull requests: %v", err)
	}

	if got := len(pullRequests); got != 101 {
		t.Fatalf("got %d pull requests, want 101", got)
	}
	if got := pullRequestNumbers(pullRequests); !reflect.DeepEqual(got, numbers) {
		t.Fatalf("got %v, want %v", got, numbers)
	}
	if *pageRequests != 2 {
		t.Fatalf("got %d page requests, want 2", *pageRequests)
	}
	if *detailRequests != 0 {
		t.Fatalf("got %d detail requests, want 0", *detailRequests)
	}
}

func TestRESTGitHubClientGetPullRequestsAvoidsOneRequestPerPullRequestBeyondOneThousand(t *testing.T) {
	t.Parallel()

	client, pageRequests, detailRequests := newPaginatedPullRequestClient(t, 1001)
	numbers := sequence(1, 1001)

	pullRequests, err := client.GetPullRequests(context.Background(), numbers)
	if err != nil {
		t.Fatalf("get pull requests: %v", err)
	}

	if got := len(pullRequests); got != 1001 {
		t.Fatalf("got %d pull requests, want 1001", got)
	}
	if got := pullRequestNumbers(pullRequests); !reflect.DeepEqual(got, numbers) {
		t.Fatalf("got %v, want %v", got[:5], numbers[:5])
	}
	if *pageRequests != 11 {
		t.Fatalf("got %d page requests, want 11", *pageRequests)
	}
	if *detailRequests != 0 {
		t.Fatalf("got %d detail requests, want 0", *detailRequests)
	}
}

func newPaginatedPullRequestClient(t *testing.T, total int) (*RESTGitHubClient, *int, *int) {
	t.Helper()

	pageRequests := 0
	detailRequests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v3/repos/octo/example/pulls":
			pageRequests++
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			if page == 0 {
				page = 1
			}
			perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
			if perPage == 0 {
				perPage = 30
			}
			if got := r.URL.Query().Get("state"); got != "closed" {
				http.Error(w, "unexpected state", http.StatusBadRequest)
				return
			}

			start := total - (page-1)*perPage
			if start <= 0 {
				_ = json.NewEncoder(w).Encode([]pullRequestDTO{})
				return
			}

			end := start - perPage + 1
			if end < 1 {
				end = 1
			}
			response := make([]pullRequestDTO, 0, start-end+1)
			for number := start; number >= end; number-- {
				response = append(response, pullRequestDTO{
					Number:  number,
					Title:   fmt.Sprintf("PR %d", number),
					HTMLURL: fmt.Sprintf("https://example.com/pulls/%d", number),
					State:   "closed",
					Merged:  true,
				})
			}
			_ = json.NewEncoder(w).Encode(response)
		case strings.HasPrefix(r.URL.Path, "/api/v3/repos/octo/example/pulls/"):
			detailRequests++
			http.Error(w, "detail endpoint should not be called", http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	parsedURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}

	return &RESTGitHubClient{
		httpClient: server.Client(),
		baseURL:    server.URL + "/api/v3/",
		repository: Repository{Host: parsedURL.Host, Scheme: parsedURL.Scheme, Owner: "octo", Name: "example"},
		token:      "dummy",
	}, &pageRequests, &detailRequests
}

func pullRequestNumbers(pullRequests []PullRequest) []int {
	numbers := make([]int, 0, len(pullRequests))
	for _, pr := range pullRequests {
		numbers = append(numbers, pr.Number)
	}
	return numbers
}

func sequence(start, end int) []int {
	values := make([]int, 0, end-start+1)
	for value := start; value <= end; value++ {
		values = append(values, value)
	}
	return values
}
