package release

import (
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	Host   string
	Scheme string
	Owner  string
	Name   string
}

func (r Repository) FullName() string {
	if r.Owner == "" || r.Name == "" {
		return ""
	}
	return r.Owner + "/" + r.Name
}

func (r Repository) APIBaseURL() string {
	if r.Host == "" {
		return "https://api.github.com/"
	}
	scheme := r.Scheme
	if scheme == "" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/api/v3/", scheme, r.Host)
}

func (r Repository) HeadRef(branch string) string {
	if r.Owner == "" {
		return branch
	}
	return r.Owner + ":" + branch
}

type User struct {
	LoginName string `json:"login_name,omitempty"`
	URL       string `json:"url,omitempty"`
	Avatar    string `json:"avatar,omitempty"`
}

type PullRequest struct {
	Number         int       `json:"number,omitempty"`
	Title          string    `json:"title,omitempty"`
	Body           string    `json:"body,omitempty"`
	URL            string    `json:"url,omitempty"`
	State          string    `json:"state,omitempty"`
	Merged         bool      `json:"merged,omitempty"`
	MergeCommitSHA string    `json:"merge_commit_sha,omitempty"`
	HeadRef        string    `json:"head_ref,omitempty"`
	BaseRef        string    `json:"base_ref,omitempty"`
	MergedAt       time.Time `json:"merged_at,omitempty"`
	User           User      `json:"user,omitempty"`
	Assignee       *User     `json:"assignee,omitempty"`
	Assignees      []User    `json:"assignees,omitempty"`
}

func (pr PullRequest) HTMLLink() string {
	return pr.URL
}

func (pr PullRequest) ToChecklistItem(printTitle bool, mentionType string) string {
	item := fmt.Sprintf("- [ ] #%d", pr.Number)
	if printTitle && pr.Title != "" {
		item += " " + pr.Title
	}
	if mention := pr.Mention(mentionType); mention != "" {
		item += " " + mention
	}
	return item
}

func (pr PullRequest) Mention(mentionType string) string {
	names := pr.TargetUserLoginNames(mentionType)
	if len(names) == 0 {
		return ""
	}
	mentions := make([]string, 0, len(names))
	for _, name := range names {
		if name == "" {
			continue
		}
		mentions = append(mentions, "@"+name)
	}
	return strings.Join(mentions, " ")
}

func (pr PullRequest) TargetUserLoginNames(mentionType string) []string {
	switch mentionType {
	case "author":
		if pr.User.LoginName == "" {
			return nil
		}
		return []string{pr.User.LoginName}
	default:
		if len(pr.Assignees) > 1 {
			names := make([]string, 0, len(pr.Assignees))
			for _, assignee := range pr.Assignees {
				if assignee.LoginName == "" {
					continue
				}
				names = append(names, assignee.LoginName)
			}
			if len(names) > 0 {
				return names
			}
		}
		if pr.Assignee != nil && pr.Assignee.LoginName != "" {
			return []string{pr.Assignee.LoginName}
		}
		if len(pr.Assignees) == 1 && pr.Assignees[0].LoginName != "" {
			return []string{pr.Assignees[0].LoginName}
		}
		if pr.User.LoginName != "" {
			return []string{pr.User.LoginName}
		}
		return nil
	}
}

type ChangedFile struct {
	Filename    string `json:"filename,omitempty"`
	Status      string `json:"status,omitempty"`
	Additions   int    `json:"additions,omitempty"`
	Deletions   int    `json:"deletions,omitempty"`
	Changes     int    `json:"changes,omitempty"`
	BlobURL     string `json:"blob_url,omitempty"`
	RawURL      string `json:"raw_url,omitempty"`
	ContentsURL string `json:"contents_url,omitempty"`
	Patch       string `json:"patch,omitempty"`
}
