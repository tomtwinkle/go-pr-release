package markdown

import (
	"bytes"
	"html/template"
	"os"
	"time"

	"github.com/google/go-github/v45/github"

	"github.com/Masterminds/sprig/v3"
)

const defaultTmpl = `# Releases
{{ range .PullRequests }}
  {{- printf "- [ ] #%d @%s" .Number .User.LoginName}}
{{ end }}
`

type TemplateParam struct {
	PullRequests []TemplateParamPullRequest
}

type TemplateParamPullRequest struct {
	Number         int
	Title          string
	MergedAt       time.Time
	MergeCommitSHA string
	User           TemplateParamUser
	URL            string
}

type TemplateParamUser struct {
	LoginName string
	URL       string
	Avatar    string
}

func MakePRBody(prs []*github.PullRequest, templatePath string) (string, error) {
	tmpPrs := make([]TemplateParamPullRequest, len(prs))
	for i, pr := range prs {
		tmpPrs[i] = TemplateParamPullRequest{
			Number:         pr.GetNumber(),
			Title:          pr.GetTitle(),
			MergedAt:       *pr.MergedAt,
			MergeCommitSHA: pr.GetMergeCommitSHA(),
			User: TemplateParamUser{
				LoginName: pr.User.GetLogin(),
				URL:       pr.User.GetHTMLURL(),
				Avatar:    pr.User.GetAvatarURL(),
			},
			URL: pr.GetURL(),
		}
	}

	var tpl *template.Template
	if templatePath != "" {
		b, err := os.ReadFile(templatePath)
		if err != nil {
			return "", err
		}
		tpl = template.Must(template.New("base").Funcs(sprig.FuncMap()).Parse(string(b)))
	} else {
		tpl = template.Must(template.New("base").Funcs(sprig.FuncMap()).Parse(defaultTmpl))
	}
	m := TemplateParam{
		PullRequests: tmpPrs,
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, m); err != nil {
		return "", err
	}
	return buf.String(), nil
}
