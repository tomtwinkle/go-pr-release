package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"strings"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	tren "github.com/go-playground/validator/v10/translations/en"
	"github.com/mkideal/cli"

	"github.com/tomtwinkle/go-pr-release/internal/markdown"
	"github.com/tomtwinkle/go-pr-release/internal/pkg/env"
	"github.com/tomtwinkle/go-pr-release/internal/pkg/gh"
)

const (
	gitDir        = ".git"
	gitRemoteName = "origin"
)

type Args struct {
	cli.Helper
	DryRun        bool     `cli:"n,dry-run" usage:"Do not create/update a PR. Just prints out" namev:"--dry-run or environment variable:GO_PR_RELEASE_DRY_RUN"`
	Token         string   `cli:"token" usage:"Token for GitHub API" validate:"required" namev:"--token or environment variable:GO_PR_RELEASE_TOKEN"`
	Title         string   `cli:"title" usage:"Title for GitHub PullRequest" namev:"--title"`
	ReleaseBranch string   `cli:"to,release-branch" usage:"Branch to be released" validate:"required" namev:"--release-branch or environment variable:GO_PR_RELEASE_RELEASE"`
	DevelopBranch string   `cli:"from,develop-branch" usage:"The Branch that will be merged into the release Branch" validate:"required" namev:"--develop-branch or environment variable:GO_PR_RELEASE_DEVELOP"`
	Template      string   `cli:"t,template" usage:"The template file path for pull requests created. This is an go template" namev:"--template or environment variable:GO_PR_RELEASE_TEMPLATE"`
	Labels        []string `cli:"l,label" usage:"The labels list for adding to pull requests created. More than one can be specified" namev:"--label or environment variable:GO_PR_RELEASE_LABELS"`
	Reviewers     []string `cli:"r,reviewer" usage:"Reviewers for pull requests. More than one can be specified" namev:"--reviewer or environment variable:GO_PR_RELEASE_REVIEWERS"`
	Verbose       bool     `cli:"verbose" usage:"Output detailed logs" namev:"--verbose"`
	Version       bool     `cli:"v,version" usage:"Version" namev:"--version"`
}

func Run(name, version, commit, date string) int {
	return cli.Run(new(Args), func(c *cli.Context) error {
		argv, ok := c.Argv().(*Args)
		if !ok {
			return fmt.Errorf("argument type mismatch [%T]", c.Argv())
		}
		if argv.Version {
			c.String(c.Color().Green(fmt.Sprintf("%s %s %s [%s]", name, version, commit, date)))
			return nil
		}

		var arg *Args
		if v, err := LookupEnv(); err != nil {
			return err
		} else {
			var bindErr error
			if arg, bindErr = BindArgs(c, v); bindErr != nil {
				return fmt.Errorf("%s", c.Color().Red(bindErr.Error()))
			}
		}
		if arg.Verbose {
			c.JSONln(arg)
		}
		return MakePR(arg)
	})
}

func LookupEnv() (*Args, error) {
	arg := new(Args)

	if dryRun, err := env.LookUpBool("GO_PR_RELEASE_DRY_RUN", false); err == nil {
		arg.DryRun = dryRun
	} else {
		return nil, err
	}
	if token, err := env.LookUpString("GO_PR_RELEASE_TOKEN", false); err == nil {
		arg.Token = token
	} else {
		return nil, err
	}
	if title, err := env.LookUpString("GO_PR_RELEASE_TITLE", false); err == nil {
		arg.Title = title
	} else {
		return nil, err
	}
	if releaseBranch, err := env.LookUpString("GO_PR_RELEASE_RELEASE", false); err == nil {
		arg.ReleaseBranch = releaseBranch
	} else {
		return nil, err
	}
	if developBranch, err := env.LookUpString("GO_PR_RELEASE_DEVELOP", false); err == nil {
		arg.DevelopBranch = developBranch
	} else {
		return nil, err
	}
	if template, err := env.LookUpString("GO_PR_RELEASE_TEMPLATE", false); err == nil {
		arg.Template = template
	} else {
		return nil, err
	}
	if labels, err := env.LookUpStringSlice("GO_PR_RELEASE_LABELS", false, ","); err == nil {
		arg.Labels = labels
	} else {
		return nil, err
	}
	if reviewers, err := env.LookUpStringSlice("GO_PR_RELEASE_REVIEWERS", false, ","); err == nil {
		arg.Reviewers = reviewers
	} else {
		return nil, err
	}

	return arg, nil
}

func BindArgs(ctx *cli.Context, arg *Args) (*Args, error) {
	argv, ok := ctx.Argv().(*Args)
	if !ok {
		return nil, fmt.Errorf("argument type mismatch [%T]", ctx.Argv())
	}
	if argv.DryRun {
		arg.DryRun = argv.DryRun
	}
	if argv.Token != "" {
		arg.Token = argv.Token
	}
	if argv.DevelopBranch != "" {
		arg.DevelopBranch = argv.DevelopBranch
	}
	if argv.ReleaseBranch != "" {
		arg.ReleaseBranch = argv.ReleaseBranch
	}
	if argv.Template != "" {
		arg.Template = argv.Template
	}
	if len(argv.Labels) > 0 {
		arg.Labels = argv.Labels
	}
	if len(argv.Reviewers) > 0 {
		arg.Reviewers = argv.Reviewers
	}
	if argv.Verbose {
		arg.Verbose = argv.Verbose
	}

	if err := ValidateArgs(arg); err != nil {
		return nil, err
	}
	return arg, nil
}

func ValidateArgs(arg *Args) error {
	const structNameTag = "namev"
	translators := map[string]string{
		"required": "{0} is required",
	}

	v := validator.New()
	uni := ut.New(en.New(), en.New())
	trans, _ := uni.GetTranslator("en")
	if err := tren.RegisterDefaultTranslations(v, trans); err != nil {
		panic(err)
	}
	v.RegisterTagNameFunc(func(fld reflect.StructField) string {
		fieldName := fld.Tag.Get(structNameTag)
		if fieldName == "-" {
			return ""
		}
		return fieldName
	})
	for tag, msgFormat := range translators {
		if err := v.RegisterTranslation(tag, trans, func(u ut.Translator) error {
			return u.Add(tag, msgFormat, true)
		}, transFunc); err != nil {
			panic(err)
		}
	}
	if err := v.Struct(arg); err != nil {
		errMessages := make([]string, 0)
		for _, m := range err.(validator.ValidationErrors).Translate(trans) {
			errMessages = append(errMessages, m)
		}
		return errors.New(strings.Join(errMessages, ","))
	}
	return nil
}

func transFunc(ut ut.Translator, fe validator.FieldError) string {
	t, err := ut.T(fe.Tag(), fe.Field(), fe.Param())
	if err != nil {
		return fe.Error()
	}
	return t
}

func MakePR(arg *Args) error {
	ctx := context.Background()

	logLevel := slog.LevelInfo
	if arg.Verbose {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	g, err := gh.New(ctx, arg.Token, logger)
	if err != nil {
		return err
	}
	mergedPRs, err := g.GetMergedPRs(ctx, arg.DevelopBranch, arg.ReleaseBranch)
	if err != nil {
		return err
	}
	body, err := markdown.MakePRBody(mergedPRs, arg.Template)
	if err != nil {
		return err
	}
	if arg.DryRun {
		fmt.Println(body)
		return nil
	}
	var title string
	if arg.Title != "" {
		title = arg.Title
	} else {
		title = fmt.Sprintf("Merge to %s from %s", arg.ReleaseBranch, arg.DevelopBranch)
	}
	pr, err := g.CreateReleasePR(ctx, title, arg.DevelopBranch, arg.ReleaseBranch, body)
	if err != nil {
		return err
	}
	if len(arg.Reviewers) > 0 {
		if _, err := g.AssignReviews(ctx, pr.GetNumber(), arg.Reviewers...); err != nil {
			return err
		}
	}
	if len(arg.Labels) > 0 {
		if _, err := g.Labeling(ctx, pr.GetNumber(), arg.Labels...); err != nil {
			return err
		}
	}
	return nil
}
