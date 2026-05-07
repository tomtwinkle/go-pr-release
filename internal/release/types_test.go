package release

import "testing"

func TestPullRequestMention(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pullRequest PullRequest
		mentionType string
		want        string
	}{
		{
			name: "multiple assignees",
			pullRequest: PullRequest{
				Assignees: []User{{LoginName: "hakobe"}, {LoginName: "toshimaru"}, {LoginName: "Copilot"}},
				User:      User{LoginName: "author"},
			},
			want: "@hakobe @toshimaru @Copilot",
		},
		{
			name: "fallback to author",
			pullRequest: PullRequest{
				User: User{LoginName: "hakobe"},
			},
			want: "@hakobe",
		},
		{
			name: "mention author explicitly",
			pullRequest: PullRequest{
				User:      User{LoginName: "hakobe"},
				Assignees: []User{{LoginName: "toshimaru"}},
			},
			mentionType: "author",
			want:        "@hakobe",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.pullRequest.Mention(tt.mentionType); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPullRequestToChecklistItem(t *testing.T) {
	t.Parallel()

	pr := PullRequest{
		Number: 7,
		Title:  "Support multiple assignees",
		Assignees: []User{
			{LoginName: "hakobe"},
			{LoginName: "toshimaru"},
			{LoginName: "Copilot"},
		},
	}

	if got, want := pr.ToChecklistItem(false, ""), "- [ ] #7 @hakobe @toshimaru @Copilot"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got, want := pr.ToChecklistItem(true, ""), "- [ ] #7 Support multiple assignees @hakobe @toshimaru @Copilot"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
