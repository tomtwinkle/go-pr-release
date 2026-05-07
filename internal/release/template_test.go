package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildTitleAndBodyDefaultTemplate(t *testing.T) {
	t.Parallel()

	title, body, err := BuildTitleAndBody(
		t.TempDir(),
		nil,
		[]PullRequest{{Number: 3, User: User{LoginName: "hakobe"}}},
		nil,
		"",
		"",
	)
	if err != nil {
		t.Fatalf("build title and body: %v", err)
	}

	if !strings.HasPrefix(title, "Release ") {
		t.Fatalf("unexpected title: %q", title)
	}
	if got, want := strings.TrimSpace(body), "- [ ] #3 @hakobe"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestBuildTitleAndBodyCustomTemplate(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	templatePath := filepath.Join(root, "template.tmpl")
	if err := os.WriteFile(templatePath, []byte(`Custom release
{{- range .pull_requests }}
{{ .ToChecklistItemWithTitle }}
{{- end }}
`), 0o600); err != nil {
		t.Fatalf("write template: %v", err)
	}

	title, body, err := BuildTitleAndBody(
		root,
		nil,
		[]PullRequest{{Number: 4, Title: "Add feature", User: User{LoginName: "alice"}}},
		nil,
		"template.tmpl",
		"",
	)
	if err != nil {
		t.Fatalf("build title and body: %v", err)
	}

	if title != "Custom release" {
		t.Fatalf("unexpected title: %q", title)
	}
	if got, want := strings.TrimSpace(body), "- [ ] #4 Add feature @alice"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestMergeBodiesPreservesChecklistState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		oldBody string
		newBody string
		want    string
	}{
		{
			name: "new pull request added",
			oldBody: `- [x] #3 Provides a creating release pull-request object for template @hakobe
- [ ] #6 Support two factor auth @ninjinkun`,
			newBody: `- [ ] #3 Provides a creating release pull-request object for template @hakobe
- [ ] #4 use user who create PR if there is no assignee @hakobe
- [ ] #6 Support two factor auth @ninjinkun`,
			want: `- [x] #3 Provides a creating release pull-request object for template @hakobe
- [ ] #4 use user who create PR if there is no assignee @hakobe
- [ ] #6 Support two factor auth @ninjinkun`,
		},
		{
			name: "new pull request added and keep task status",
			oldBody: `- [x] #4 use user who create PR if there is no assignee @hakobe
- [x] #6 Support two factor auth @ninjinkun`,
			newBody: `- [ ] #3 Provides a creating release pull-request object for template @hakobe
- [ ] #4 use user who create PR if there is no assignee @hakobe
- [ ] #6 Support two factor auth @ninjinkun`,
			want: `- [ ] #3 Provides a creating release pull-request object for template @hakobe
- [x] #4 use user who create PR if there is no assignee @hakobe
- [x] #6 Support two factor auth @ninjinkun`,
		},
		{
			name: "same number appears later",
			oldBody: `- [x] #3 Provides a creating release pull-request object for template @hakobe
- [ ] #6 Support two factor auth @ninjinkun`,
			newBody: `- [ ] #3 Provides a creating release pull-request object for template @hakobe
- [ ] #4 use user who create PR if there is no assignee @hakobe
- [ ] #6 Support two factor auth @ninjinkun
- [ ] #30 Extract logic from bin/git-pr-release @banyan`,
			want: `- [x] #3 Provides a creating release pull-request object for template @hakobe
- [ ] #4 use user who create PR if there is no assignee @hakobe
- [ ] #6 Support two factor auth @ninjinkun
- [ ] #30 Extract logic from bin/git-pr-release @banyan`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := MergeBodies(tt.oldBody, tt.newBody); got != tt.want {
				t.Fatalf("got:\n%s\n\nwant:\n%s", got, tt.want)
			}
		})
	}
}
