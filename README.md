# go-pr-release

`x-motemen/git-pr-release` の主要フローを Go で書き直した CLI です。`git-pr-release` 互換の設定系を優先しつつ、既存の `go-pr-release` 向け環境変数も後方互換で受け付けます。

## Compatibility

- merge commit ベースの PR 収集
- `--squashed` による squash merge PR 収集
- 既存 release PR の再利用と checklist 状態の引き継ぎ
- `--no-fetch`, `--dry-run`, `--json`, `--overwrite-description`
- `pr-release.*` git config / `.git-pr-release`
- GitHub Enterprise remote の API endpoint 解決
- `--assign-pr-author`, `--request-pr-author-review`, `--mention author`

## Configuration

優先順位は **CLI option > 環境変数 > `.git-pr-release` / git config > default** です。

### Environment variables

| Primary | Legacy alias | Description |
|---|---|---|
| `GIT_PR_RELEASE_TOKEN` | `GO_PR_RELEASE_TOKEN` | Required. GitHub token |
| `GIT_PR_RELEASE_BRANCH_PRODUCTION` | `GO_PR_RELEASE_RELEASE` | Production branch. Default: `master` |
| `GIT_PR_RELEASE_BRANCH_STAGING` | `GO_PR_RELEASE_DEVELOP` | Staging branch. Default: `staging` |
| `GIT_PR_RELEASE_TEMPLATE` | `GO_PR_RELEASE_TEMPLATE` | Go template path |
| `GIT_PR_RELEASE_LABELS` | `GO_PR_RELEASE_LABELS` | Comma-separated labels |
| `GIT_PR_RELEASE_REVIEWERS` | `GO_PR_RELEASE_REVIEWERS` | Comma-separated extra reviewers |
| `GIT_PR_RELEASE_TITLE` | `GO_PR_RELEASE_TITLE` | Explicit release PR title |
| `GIT_PR_RELEASE_DRY_RUN` | `GO_PR_RELEASE_DRY_RUN` | Dry-run toggle |
| `GIT_PR_RELEASE_MENTION` | - | `author` を指定すると author mention を使う |
| `GIT_PR_RELEASE_ASSIGN_PR_AUTHOR` | - | `true` / `false` |
| `GIT_PR_RELEASE_REQUEST_PR_AUTHOR_REVIEW` | - | `true` / `false` |
| `GIT_PR_RELEASE_SSL_NO_VERIFY` | - | GitHub Enterprise で証明書検証を無効化 |

### CLI options

| Option | Description |
|---|---|
| `--token` | GitHub token |
| `--production-branch`, `--release-branch`, `--to` | Production branch |
| `--staging-branch`, `--develop-branch`, `--from` | Staging branch |
| `--template`, `-t` | Template path |
| `--label`, `-l` | Labels |
| `--reviewer`, `-r` | Extra reviewers |
| `--title` | Release PR title override |
| `--mention` | Mention strategy (`author`) |
| `--assign-pr-author` | Assign merged PR authors/assignees |
| `--request-pr-author-review` | Request review from merged PR authors/assignees |
| `--dry-run`, `-n` | Do not create/update PR |
| `--json` | Print release payload as JSON |
| `--no-fetch` | Skip `git remote update origin` |
| `--squashed` | Include squash merged PRs |
| `--overwrite-description` | Do not merge checklist state from existing body |
| `--verbose` | Print resolved runtime configuration |
| `--version`, `-v` | Print version |

### git config keys

`.git-pr-release` では `pr-release.*` をそのまま指定できます。

```ini
[pr-release]
token = ghp_xxx
branch.production = main
branch.staging = develop
template = .github/template.tmpl
labels = release,qa
mention = author
assign-pr-author = true
request-pr-author-review = true
```

GitHub Enterprise の場合は host-aware な git config も使えます。

```bash
git config pr-release.ghe.example.com.branch.staging develop
```

## GitHub Actions

```yaml
name: go-pr-release

on:
  push:
    branches:
      - develop

jobs:
  go-pr-release:
    runs-on: ubuntu-latest

    steps:
      - uses: actions/checkout@v3
        with:
          fetch-depth: 0

      - name: Install go-pr-release
        run: curl -s -L https://github.com/tomtwinkle/go-pr-release/releases/latest/download/go-pr-release_linux_x86_64.tar.gz | tar -xvz

      - name: Run
        env:
          GIT_PR_RELEASE_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          GIT_PR_RELEASE_BRANCH_PRODUCTION: main
          GIT_PR_RELEASE_BRANCH_STAGING: develop
          GIT_PR_RELEASE_LABELS: release
          GIT_PR_RELEASE_TEMPLATE: .github/template.tmpl
          GIT_PR_RELEASE_ASSIGN_PR_AUTHOR: true
          GIT_PR_RELEASE_REQUEST_PR_AUTHOR_REVIEW: true
        run: ./go-pr-release --squashed
```

## Template

テンプレートは **1 行目が title、2 行目以降が body** です。Go template と sprig functions を使えます。

利用できる代表的な値:

```go
type TemplateData struct {
	ReleasePullRequest PullRequest
	TargetPullRequest  PullRequest
	MergedPullRequests []PullRequest
	PullRequests       []PullRequest
	ChangedFiles       []ChangedFile
}

type PullRequest struct {
	Number         int
	Title          string
	MergedAt       time.Time
	MergeCommitSHA string
	User           User
	URL            string
}

type User struct {
	LoginName string
	URL       string
	Avatar    string
}
```

`PullRequests` のほかに、`pull_requests` / `merged_pull_requests` / `release_pull_request` / `target_pull_request` / `changed_files` も使えます。

サンプルテンプレート:

```gotemplate
Release {{ now | date "2006-01-02" }}

# Releases
{{- range .PullRequests }}
{{ printf "- [ ] [%s] #%d @%s" (.MergedAt.Format "2006-01-02") .Number .User.LoginName }}
{{- end }}
```

## Exit status

- `0`: success
- `1`: release 対象 PR がない、または実行時エラー
