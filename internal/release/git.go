package release

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const DefaultRemoteName = "origin"

var prRefPattern = regexp.MustCompile(`^refs/pull/(\d+)/head$`)

type Git struct {
	Dir string
}

func NewGit(dir string) *Git {
	if dir == "" {
		dir = "."
	}
	return &Git{Dir: dir}
}

func (g *Git) Output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), message)
	}

	return strings.TrimRight(stdout.String(), "\n"), nil
}

func (g *Git) Lines(ctx context.Context, args ...string) ([]string, error) {
	output, err := g.Output(ctx, args...)
	if err != nil {
		return nil, err
	}
	if output == "" {
		return nil, nil
	}
	rawLines := strings.Split(strings.ReplaceAll(output, "\r\n", "\n"), "\n")
	lines := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
}

func (g *Git) Root(ctx context.Context) (string, error) {
	return g.Output(ctx, "rev-parse", "--show-toplevel")
}

func (g *Git) ResolveRemote(ctx context.Context, remoteName string) (Repository, error) {
	if remoteName == "" {
		remoteName = DefaultRemoteName
	}
	remoteURL, ok, err := g.LookupConfig(ctx, "remote."+remoteName+".url")
	if err != nil {
		return Repository{}, err
	}
	if !ok || remoteURL == "" {
		return Repository{}, fmt.Errorf("git remote %q is not configured", remoteName)
	}
	return ParseRemoteURL(remoteURL)
}

func ParseRemoteURL(raw string) (Repository, error) {
	remote := strings.TrimSpace(raw)
	if remote == "" {
		return Repository{}, errors.New("remote url is empty")
	}

	if !strings.Contains(remote, "://") && strings.Contains(remote, ":") {
		remote = "ssh://" + strings.Replace(remote, ":", "/", 1)
	}

	u, err := parseURL(remote)
	if err != nil {
		return Repository{}, err
	}

	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return Repository{}, fmt.Errorf("unexpected remote path %q", raw)
	}

	host := u.Hostname()
	scheme := u.Scheme
	if scheme == "" || scheme == "ssh" {
		scheme = "https"
	}
	if host == "github.com" {
		host = ""
		scheme = "https"
	}

	return Repository{
		Host:   host,
		Scheme: scheme,
		Owner:  parts[0],
		Name:   parts[1],
	}, nil
}

func parseURL(raw string) (*urlAdapter, error) {
	u, err := newURLAdapter(raw)
	if err != nil {
		return nil, fmt.Errorf("parse remote url %q: %w", raw, err)
	}
	return u, nil
}

func (g *Git) LookupProjectConfig(ctx context.Context, repo Repository, key string) (string, bool, error) {
	root, err := g.Root(ctx)
	if err != nil {
		return "", false, err
	}

	projectConfigPath := filepath.Join(root, ".git-pr-release")
	if _, err := os.Stat(projectConfigPath); err == nil {
		value, ok, cfgErr := g.lookupConfigWithArgs(ctx, "-f", projectConfigPath, "pr-release."+key)
		if cfgErr != nil {
			return "", false, cfgErr
		}
		if ok {
			return value, true, nil
		}
	}

	hostAwareKey := "pr-release." + key
	if repo.Host != "" {
		hostAwareKey = "pr-release." + repo.Host + "." + key
	}
	return g.LookupConfig(ctx, hostAwareKey)
}

func (g *Git) LookupConfig(ctx context.Context, key string) (string, bool, error) {
	return g.lookupConfigWithArgs(ctx, key)
}

func (g *Git) lookupConfigWithArgs(ctx context.Context, args ...string) (string, bool, error) {
	cmdArgs := append([]string{"config"}, args...)

	cmd := exec.CommandContext(ctx, "git", cmdArgs...)
	cmd.Dir = g.Dir

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", false, nil
		}
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return "", false, fmt.Errorf("git %s: %s", strings.Join(cmdArgs, " "), message)
	}

	return strings.TrimSpace(stdout.String()), true, nil
}

func (g *Git) IsShallow(ctx context.Context) (bool, error) {
	value, err := g.Output(ctx, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

func (g *Git) Unshallow(ctx context.Context) error {
	_, err := g.Output(ctx, "fetch", "--unshallow")
	return err
}

func (g *Git) RemoteUpdate(ctx context.Context, remoteName string) error {
	if remoteName == "" {
		remoteName = DefaultRemoteName
	}
	_, err := g.Output(ctx, "remote", "update", remoteName)
	return err
}

func (g *Git) MergedPRNumbers(ctx context.Context, remoteName, productionBranch, stagingBranch string) ([]int, error) {
	if remoteName == "" {
		remoteName = DefaultRemoteName
	}

	mergeLines, err := g.Lines(
		ctx,
		"log",
		"--merges",
		"--pretty=format:%P",
		fmt.Sprintf("%s/%s..%s/%s", remoteName, productionBranch, remoteName, stagingBranch),
	)
	if err != nil {
		return nil, err
	}

	featureShas := make(map[string]struct{}, len(mergeLines))
	for _, line := range mergeLines {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		featureShas[fields[1]] = struct{}{}
	}

	refLines, err := g.Lines(ctx, "ls-remote", remoteName, "refs/pull/*/head")
	if err != nil {
		return nil, err
	}

	var numbers []int
	seen := map[int]struct{}{}
	for _, line := range refLines {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		sha := fields[0]
		ref := fields[1]
		if _, ok := featureShas[sha]; !ok {
			continue
		}
		number, ok := parsePullRequestRef(ref)
		if !ok {
			continue
		}

		mergeBase, err := g.Output(ctx, "merge-base", sha, fmt.Sprintf("%s/%s", remoteName, productionBranch))
		if err != nil {
			return nil, err
		}
		if mergeBase == sha {
			continue
		}

		if _, exists := seen[number]; exists {
			continue
		}
		seen[number] = struct{}{}
		numbers = append(numbers, number)
	}

	return numbers, nil
}

func (g *Git) SquashCommitSHAs(ctx context.Context, remoteName, productionBranch, stagingBranch string) ([]string, error) {
	if remoteName == "" {
		remoteName = DefaultRemoteName
	}
	return g.Lines(
		ctx,
		"log",
		"--pretty=format:%h",
		"--abbrev=7",
		"--no-merges",
		"--first-parent",
		fmt.Sprintf("%s/%s..%s/%s", remoteName, productionBranch, remoteName, stagingBranch),
	)
}

func parsePullRequestRef(ref string) (int, bool) {
	matches := prRefPattern.FindStringSubmatch(ref)
	if len(matches) != 2 {
		return 0, false
	}
	number, err := strconv.Atoi(matches[1])
	if err != nil {
		return 0, false
	}
	return number, true
}

type urlAdapter struct {
	Scheme string
	Host   string
	Path   string
}

func (u *urlAdapter) Hostname() string {
	host := u.Host
	if idx := strings.Index(host, "@"); idx >= 0 {
		host = host[idx+1:]
	}
	if idx := strings.Index(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	return host
}

func newURLAdapter(raw string) (*urlAdapter, error) {
	parts := strings.SplitN(raw, "://", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("missing scheme in %q", raw)
	}
	scheme := parts[0]
	rest := parts[1]
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return nil, fmt.Errorf("missing path in %q", raw)
	}
	return &urlAdapter{
		Scheme: scheme,
		Host:   rest[:slash],
		Path:   rest[slash:],
	}, nil
}
