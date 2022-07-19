# go-pr-release
CLI for creating PullRequest for release in Github Action.
it was respected by [git-pr-release](https://github.com/x-motemen/git-pr-release) and rewritten in Golang.
since it runs in one binary, it is expected to run quickly on CI.

![image](https://user-images.githubusercontent.com/47764757/179726677-2d5ee674-6f7a-4d3c-9c18-c7a979a8f25b.png)

## Configuration
The git configuration is read from `.git/config`.
It works directly under the directory from which you `git clone`.

| Environment Variables | CLI Option | Description |
|---|---|---|
| GO_PR_RELEASE_TOKEN     | --token                  | Required `secrets.GITHUB_TOKEN` or a personal token with repo privileges |
| GO_PR_RELEASE_RELEASE   | --release-branch, --to   | Required. Release Branch: Destination to be merged |
| GO_PR_RELEASE_DEVELOP   | --develop-branch, --from | Required. Develop Branch: Merge source |
| GO_PR_RELEASE_LABELS    | --label, -l              | Optional. PullRequest labels. Multiple labels can be specified, separated by `commas` |
| GO_PR_RELEASE_REVIEWERS | --reviewer, -r           | Optional. PullRequest reviewers. Multiple reviewers can be specified, separated by `commas` |
| GO_PR_RELEASE_TITLE     | --title                  | Optional. specify the title of the pull request |
| GO_PR_RELEASE_TEMPLATE  | --template, -t           | Optional. Specify a template file that can be described in `go template` |
| GO_PR_RELEASE_DRY_RUN   | --dry-run, -n            | Optional. if true, display only the results to be created without creating PullRequest |
|                         | --verbose                | Optional. Detailed logs will be output. Do not specify except for verification |
|                         | --version, -v            | Optional. Output CLI version information |

### Installation in Github Action

```yaml
name: go-pr-release

on:
  push:
    branches:
      - develop
      - enhancement/test-ci-go-pr-release

jobs:
  test:
    name: go-pr-release
    runs-on: ubuntu-latest

    steps:
      - name: Checkout
        uses: actions/checkout@v2

      - name: Set up Go
        uses: actions/setup-go@v3
        with:
          go-version: 1.18

      - name: Install go-pr-release
        env:
          VERSION: 0.1.0
        run: curl -L https://github.com/tomtwinkle/go-pr-release/releases/download/v${VERSION}/go-pr-release_${VERSION}_linux_amd64.tar.gz | tar -xz

      - name: Run
        env:
          GO_PR_RELEASE_TOKEN: ${{ secrets.GITHUB_TOKEN }} # Required
          GO_PR_RELEASE_RELEASE: main                      # Required. Release Branch: Destination to be merged
          GO_PR_RELEASE_DEVELOP: develop                   # Required. Develop Branch: Merge source
          GO_PR_RELEASE_LABELS: release                    # Optional. PullRequest labels. Multiple labels can be specified, separated by `commas`
          GO_PR_RELEASE_TITLE: ""                          # Optional. specify the title of the pull request
          GO_PR_RELEASE_TEMPLATE: ".github/template.tmpl"  # Optional. Specify a template file that can be described in `go template`
          GO_PR_RELEASE_REVIEWERS: tomtwinkle              # Optional. PullRequest reviewers. Multiple reviewers can be specified, separated by `commas`
          GO_PR_RELEASE_DRY_RUN: false                     # Optional. if true, display only the results to be created without creating PullRequest
        run: ./go-pr-release
```


### Template

A `go template` file can be specified for template.
Custom [sprig functions](https://github.com/Masterminds/sprig) can be used for `go template`.

The following structs are available as go template parameters.

```go
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
```

### Example

- [Example Github Action Workflow](https://github.com/tomtwinkle/go-pr-release-test/tree/develop/.github)
- [Example PullRequest](https://github.com/tomtwinkle/go-pr-release-test/pull/16)
