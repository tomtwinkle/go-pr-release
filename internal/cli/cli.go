package cli

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	tren "github.com/go-playground/validator/v10/translations/en"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/mkideal/cli"
	"github.com/tomtwinkle/go-pr-release/internal/pkg/env"
)

type Args struct {
	cli.Helper
	DryRun        bool     `cli:"n,dry-run" usage:"Do not create/update a PR. Just prints out" namev:"--dry-run or environment variable:GO_PR_RELEASE_DRY_RUN"`
	Token         string   `cli:"token" usage:"Token for GitHub API" validate:"required" namev:"--token or environment variable:GO_PR_RELEASE_TOKEN"`
	ReleaseBranch string   `cli:"to,release-branch" usage:"Branch to be released" validate:"required" namev:"--release-branch or environment variable:GO_PR_RELEASE_RELEASE"`
	DevelopBranch string   `cli:"from,develop-branch" usage:"The Branch that will be merged into the release Branch" validate:"required" namev:"--develop-branch or environment variable:GO_PR_RELEASE_DEVELOP"`
	Template      string   `cli:"t,template" usage:"The template file path for pull requests created. This is an go template" namev:"--template or environment variable:GO_PR_RELEASE_TEMPLATE"`
	Labels        []string `cli:"l,label" usage:"The labels list for adding to pull requests created. More than one can be specified" namev:"--label or environment variable:GO_PR_RELEASE_LABELS"`
	Reviewers     []string `cli:"r,reviewer" usage:"Reviewers for pull requests. More than one can be specified" namev:"--reviewer or environment variable:GO_PR_RELEASE_REVIEWERS"`
	Verbose       bool     `cli:"verbose" usage:"Output detailed logs" namev:"--verbose"`
}

func Run() int {
	return cli.Run(new(Args), func(ctx *cli.Context) error {
		var arg *Args
		if v, err := LookupEnv(); err != nil {
			return err
		} else {
			var bindErr error
			if arg, bindErr = BindArgs(ctx, v); bindErr != nil {
				return fmt.Errorf("%s", ctx.Color().Red(bindErr.Error()))
			}
		}
		if arg.Verbose {
			ctx.JSONln(arg)
		}
		return nil
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
