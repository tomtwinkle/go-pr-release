package markdown

import (
	"bytes"
	"context"
	"html/template"
	"time"

	"github.com/Masterminds/sprig/v3"

	"github.com/tomtwinkle/go-pr-release/internal/cli"
	"github.com/tomtwinkle/go-pr-release/internal/pkg/gh"
)

const (
	gitDir        = ".git"
	gitRemoteName = "origin"
)

const defaultTmpl = `# Releases
{{ range .PullRequests }}
  {{- printf "- [ ] #%d @%s" .Number .User.LoginName}}
{{ end }}
`

type TemplateParam struct {
	PullRequests  []TemplateParamPullRequest
	DevelopBranch string
	ReleaseBranch string
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

type PullRequest interface {
	MakePRBody(context.Context, cli.Args) (string, error)
}

type pullRequest struct {
	gh gh.Github
}

func New(ctx context.Context, token string) (PullRequest, error) {
	g, err := gh.New(ctx, token, gh.RemoteConfigParam{
		GitDirPath: gitDir,
		RemoteName: gitRemoteName,
	})
	if err != nil {
		return nil, err
	}
	return &pullRequest{gh: g}, nil
}

func NewWithConfig(ctx context.Context, token string, config gh.RemoteConfig) PullRequest {
	g := gh.NewWithConfig(ctx, token, config)
	return &pullRequest{gh: g}
}

func (p *pullRequest) MakePRBody(ctx context.Context, args cli.Args) (string, error) {
	prs, err := p.gh.GetMergedPRs(ctx, args.DevelopBranch, args.ReleaseBranch)
	if err != nil {
		return "", err
	}
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
	if args.Template != "" {
		tpl = template.Must(template.New("base").Funcs(sprig.FuncMap()).ParseFiles(args.Template))
	} else {
		tpl = template.Must(template.New("base").Funcs(sprig.FuncMap()).Parse(defaultTmpl))
	}
	m := TemplateParam{
		PullRequests:  tmpPrs,
		DevelopBranch: args.DevelopBranch,
		ReleaseBranch: args.ReleaseBranch,
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, m); err != nil {
		return "", err
	}
	return buf.String(), nil
}
