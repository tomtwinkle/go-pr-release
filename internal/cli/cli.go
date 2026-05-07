package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/tomtwinkle/go-pr-release/internal/release"
)

type serviceRunner interface {
	Run(context.Context) error
}

type CommandOptions struct {
	Name       string
	Version    string
	Commit     string
	Date       string
	Args       []string
	WorkDir    string
	Stdout     io.Writer
	Stderr     io.Writer
	LookupEnv  func(string) (string, bool)
	NewService func(release.Config, io.Writer, io.Writer) serviceRunner
}

func Run(name, version, commit, date string) int {
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	return ExecuteContext(context.Background(), CommandOptions{
		Name:      name,
		Version:   version,
		Commit:    commit,
		Date:      date,
		Args:      os.Args[1:],
		WorkDir:   workDir,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		LookupEnv: os.LookupEnv,
		NewService: func(config release.Config, stdout io.Writer, stderr io.Writer) serviceRunner {
			return release.NewService(config, stdout, stderr)
		},
	})
}

func ExecuteContext(ctx context.Context, options CommandOptions) int {
	if options.Stdout == nil {
		options.Stdout = io.Discard
	}
	if options.Stderr == nil {
		options.Stderr = io.Discard
	}
	if options.LookupEnv == nil {
		options.LookupEnv = os.LookupEnv
	}
	if options.NewService == nil {
		options.NewService = func(config release.Config, stdout io.Writer, stderr io.Writer) serviceRunner {
			return release.NewService(config, stdout, stderr)
		}
	}
	if options.WorkDir == "" {
		options.WorkDir = "."
	}

	parsed, err := parseArgs(options.Args, options.Stderr)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(options.Stderr, err)
		return 2
	}

	if parsed.version.value {
		fmt.Fprintf(options.Stdout, "%s %s %s [%s]\n", options.Name, options.Version, options.Commit, options.Date)
		return 0
	}

	config, err := resolveConfig(ctx, options.WorkDir, options.LookupEnv, parsed)
	if err != nil {
		fmt.Fprintln(options.Stderr, err)
		return 1
	}

	if config.Verbose {
		fmt.Fprintf(options.Stderr, "repository=%s production=%s staging=%s template=%s\n", config.Repository.FullName(), config.ProductionBranch, config.StagingBranch, config.TemplatePath)
	}

	service := options.NewService(config, options.Stdout, options.Stderr)
	if err := service.Run(ctx); err != nil {
		if errors.Is(err, release.ErrNoPullRequestsToRelease) {
			return 1
		}
		fmt.Fprintln(options.Stderr, err)
		return 1
	}

	return 0
}

type parsedArgs struct {
	token                 stringOption
	title                 stringOption
	productionBranch      stringOption
	stagingBranch         stringOption
	templatePath          stringOption
	labels                stringSliceOption
	reviewers             stringSliceOption
	mention               stringOption
	assignPRAuthor        boolOption
	requestPRAuthorReview boolOption
	dryRun                boolOption
	json                  boolOption
	noFetch               boolOption
	squashed              boolOption
	overwriteDescription  boolOption
	verbose               boolOption
	version               boolOption
}

func parseArgs(args []string, stderr io.Writer) (parsedArgs, error) {
	var parsed parsedArgs

	flagSet := flag.NewFlagSet("go-pr-release", flag.ContinueOnError)
	flagSet.SetOutput(stderr)
	flagSet.Usage = func() {
		fmt.Fprintln(stderr, "Usage: go-pr-release [options]")
		flagSet.PrintDefaults()
	}

	flagSet.Var(&parsed.token, "token", "GitHub API token")
	flagSet.Var(&parsed.title, "title", "Release pull request title")

	flagSet.Var(&parsed.productionBranch, "production-branch", "Production branch")
	flagSet.Var(&parsed.productionBranch, "release-branch", "Production branch")
	flagSet.Var(&parsed.productionBranch, "to", "Production branch")

	flagSet.Var(&parsed.stagingBranch, "staging-branch", "Staging branch")
	flagSet.Var(&parsed.stagingBranch, "develop-branch", "Staging branch")
	flagSet.Var(&parsed.stagingBranch, "from", "Staging branch")

	flagSet.Var(&parsed.templatePath, "template", "Template file path")
	flagSet.Var(&parsed.templatePath, "t", "Template file path")

	flagSet.Var(&parsed.labels, "label", "Labels to add")
	flagSet.Var(&parsed.labels, "l", "Labels to add")

	flagSet.Var(&parsed.reviewers, "reviewer", "Reviewers to request")
	flagSet.Var(&parsed.reviewers, "r", "Reviewers to request")

	flagSet.Var(&parsed.mention, "mention", "Mention target (author)")
	flagSet.Var(&parsed.assignPRAuthor, "assign-pr-author", "Assign PR authors to the release PR")
	flagSet.Var(&parsed.requestPRAuthorReview, "request-pr-author-review", "Request review from PR authors")

	flagSet.Var(&parsed.dryRun, "dry-run", "Do not create or update the release PR")
	flagSet.Var(&parsed.dryRun, "n", "Do not create or update the release PR")
	flagSet.Var(&parsed.json, "json", "Print release payload as JSON")
	flagSet.Var(&parsed.noFetch, "no-fetch", "Do not update origin before inspection")
	flagSet.Var(&parsed.squashed, "squashed", "Include squash merged pull requests")
	flagSet.Var(&parsed.overwriteDescription, "overwrite-description", "Overwrite the release PR description instead of merging checklists")
	flagSet.Var(&parsed.verbose, "verbose", "Print verbose logs")
	flagSet.Var(&parsed.version, "version", "Print version")
	flagSet.Var(&parsed.version, "v", "Print version")

	if err := flagSet.Parse(args); err != nil {
		return parsed, err
	}
	if flagSet.NArg() > 0 {
		return parsed, fmt.Errorf("unexpected arguments: %s", strings.Join(flagSet.Args(), " "))
	}

	return parsed, nil
}

func resolveConfig(
	ctx context.Context,
	workDir string,
	lookupEnv func(string) (string, bool),
	args parsedArgs,
) (release.Config, error) {
	git := release.NewGit(workDir)
	repository, err := git.ResolveRemote(ctx, release.DefaultRemoteName)
	if err != nil {
		return release.Config{}, err
	}

	gitString := func(key string) (string, error) {
		value, ok, err := git.LookupProjectConfig(ctx, repository, key)
		if err != nil || !ok {
			return "", err
		}
		return strings.TrimSpace(value), nil
	}

	gitBool := func(key string) (bool, bool, error) {
		value, err := gitString(key)
		if err != nil || value == "" {
			return false, false, err
		}
		parsedValue, parseErr := strconv.ParseBool(value)
		if parseErr != nil {
			return false, false, fmt.Errorf("parse git config %s: %w", key, parseErr)
		}
		return parsedValue, true, nil
	}

	config := release.Config{
		WorkDir:    workDir,
		RemoteName: release.DefaultRemoteName,
		Repository: repository,
	}

	config.Token, err = pickString(args.token, lookupEnv, gitString, "token", []string{"GIT_PR_RELEASE_TOKEN", "GO_PR_RELEASE_TOKEN"}, "")
	if err != nil {
		return release.Config{}, err
	}
	config.Title, err = pickString(args.title, lookupEnv, gitString, "", []string{"GIT_PR_RELEASE_TITLE", "GO_PR_RELEASE_TITLE"}, "")
	if err != nil {
		return release.Config{}, err
	}
	config.ProductionBranch, err = pickString(args.productionBranch, lookupEnv, gitString, "branch.production", []string{"GIT_PR_RELEASE_BRANCH_PRODUCTION", "GO_PR_RELEASE_RELEASE"}, "master")
	if err != nil {
		return release.Config{}, err
	}
	config.StagingBranch, err = pickString(args.stagingBranch, lookupEnv, gitString, "branch.staging", []string{"GIT_PR_RELEASE_BRANCH_STAGING", "GO_PR_RELEASE_DEVELOP"}, "staging")
	if err != nil {
		return release.Config{}, err
	}
	config.TemplatePath, err = pickString(args.templatePath, lookupEnv, gitString, "template", []string{"GIT_PR_RELEASE_TEMPLATE", "GO_PR_RELEASE_TEMPLATE"}, "")
	if err != nil {
		return release.Config{}, err
	}
	config.Mention, err = pickString(args.mention, lookupEnv, gitString, "mention", []string{"GIT_PR_RELEASE_MENTION"}, "")
	if err != nil {
		return release.Config{}, err
	}

	config.Labels, err = pickStringSlice(args.labels, lookupEnv, gitString, "labels", []string{"GIT_PR_RELEASE_LABELS", "GO_PR_RELEASE_LABELS"})
	if err != nil {
		return release.Config{}, err
	}
	config.ExtraReviewers, err = pickStringSlice(args.reviewers, lookupEnv, gitString, "", []string{"GIT_PR_RELEASE_REVIEWERS", "GO_PR_RELEASE_REVIEWERS"})
	if err != nil {
		return release.Config{}, err
	}

	config.AssignPRAuthor, err = pickBool(args.assignPRAuthor, lookupEnv, gitBool, "assign-pr-author", []string{"GIT_PR_RELEASE_ASSIGN_PR_AUTHOR"}, false)
	if err != nil {
		return release.Config{}, err
	}
	config.RequestPRAuthorReview, err = pickBool(args.requestPRAuthorReview, lookupEnv, gitBool, "request-pr-author-review", []string{"GIT_PR_RELEASE_REQUEST_PR_AUTHOR_REVIEW"}, false)
	if err != nil {
		return release.Config{}, err
	}
	config.DryRun, err = pickBool(args.dryRun, lookupEnv, gitBool, "", []string{"GIT_PR_RELEASE_DRY_RUN", "GO_PR_RELEASE_DRY_RUN"}, false)
	if err != nil {
		return release.Config{}, err
	}
	config.InsecureSkipTLSVerify, err = pickBool(boolOption{}, lookupEnv, gitBool, "ssl-no-verify", []string{"GIT_PR_RELEASE_SSL_NO_VERIFY"}, false)
	if err != nil {
		return release.Config{}, err
	}

	config.JSON = args.json.value
	config.NoFetch = args.noFetch.value
	config.Squashed = args.squashed.value
	config.OverwriteDescription = args.overwriteDescription.value
	config.Verbose = args.verbose.value

	if config.Token == "" {
		return release.Config{}, errors.New("token is required (--token, GIT_PR_RELEASE_TOKEN, GO_PR_RELEASE_TOKEN, or pr-release.token)")
	}

	return config, nil
}

func pickString(
	option stringOption,
	lookupEnv func(string) (string, bool),
	gitConfig func(string) (string, error),
	gitKey string,
	envKeys []string,
	defaultValue string,
) (string, error) {
	if option.set {
		return strings.TrimSpace(option.value), nil
	}
	for _, key := range envKeys {
		if value, ok := lookupEnv(key); ok {
			return strings.TrimSpace(value), nil
		}
	}
	if gitKey != "" {
		value, err := gitConfig(gitKey)
		if err != nil {
			return "", err
		}
		if value != "" {
			return value, nil
		}
	}
	return defaultValue, nil
}

func pickStringSlice(
	option stringSliceOption,
	lookupEnv func(string) (string, bool),
	gitConfig func(string) (string, error),
	gitKey string,
	envKeys []string,
) ([]string, error) {
	if option.set {
		return option.values, nil
	}
	for _, key := range envKeys {
		if value, ok := lookupEnv(key); ok {
			return splitCommaSeparated(value), nil
		}
	}
	if gitKey != "" {
		value, err := gitConfig(gitKey)
		if err != nil {
			return nil, err
		}
		if value != "" {
			return splitCommaSeparated(value), nil
		}
	}
	return nil, nil
}

func pickBool(
	option boolOption,
	lookupEnv func(string) (string, bool),
	gitConfig func(string) (bool, bool, error),
	gitKey string,
	envKeys []string,
	defaultValue bool,
) (bool, error) {
	if option.set {
		return option.value, nil
	}
	for _, key := range envKeys {
		if value, ok := lookupEnv(key); ok {
			parsedValue, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				return false, fmt.Errorf("parse environment variable %s: %w", key, err)
			}
			return parsedValue, nil
		}
	}
	if gitKey != "" {
		value, ok, err := gitConfig(gitKey)
		if err != nil {
			return false, err
		}
		if ok {
			return value, nil
		}
	}
	return defaultValue, nil
}

func splitCommaSeparated(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		values = append(values, part)
	}
	return values
}

type stringOption struct {
	value string
	set   bool
}

func (o *stringOption) String() string {
	return o.value
}

func (o *stringOption) Set(value string) error {
	o.value = value
	o.set = true
	return nil
}

type stringSliceOption struct {
	values []string
	set    bool
}

func (o *stringSliceOption) String() string {
	return strings.Join(o.values, ",")
}

func (o *stringSliceOption) Set(value string) error {
	o.set = true
	o.values = append(o.values, splitCommaSeparated(value)...)
	return nil
}

type boolOption struct {
	value bool
	set   bool
}

func (o *boolOption) String() string {
	return strconv.FormatBool(o.value)
}

func (o *boolOption) Set(value string) error {
	parsedValue, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	o.value = parsedValue
	o.set = true
	return nil
}

func (o *boolOption) IsBoolFlag() bool {
	return true
}
