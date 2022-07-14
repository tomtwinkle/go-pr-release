package cli

import (
	"errors"
	"fmt"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	tren "github.com/go-playground/validator/v10/translations/en"
	"github.com/mkideal/cli"
	"github.com/tomtwinkle/go-pr-release/internal/pkg/env"
)

type Args struct {
	cli.Helper
	DryRun        bool     `cli:"n,dry-run" usage:"Do not create/update a PR. Just prints out"`
	Token         string   `cli:"token" usage:"Token for GitHub API" validate:"required"`
	ReleaseBranch string   `cli:"to,release-branch" usage:"Branch to be released" validate:"required"`
	DevelopBranch string   `cli:"from,develop-branch" usage:"The Branch that will be merged into the release Branch" validate:"required"`
	Template      string   `cli:"t,template" usage:"The template file path for pull requests created. This is an go template"`
	Labels        []string `cli:"l,label" usage:"The labels list for adding to pull requests created. More than one can be specified"`
	Reviewers     []string `cli:"r,reviewer" usage:"Reviewers for pull requests. More than one can be specified"`
	Verbose       bool     `cli:"verbose" usage:"Output detailed logs"`
}

func Run() int {
	return cli.Run(new(Args), func(ctx *cli.Context) error {
		var arg *Args
		if v, err := LookupEnv(); err != nil {
			return err
		} else {
			var bindErr error
			if arg, bindErr = BindArgs(ctx, v); bindErr != nil {
				return bindErr
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
	v := validator.New()
	jaEn := en.New()
	uni := ut.New(jaEn, jaEn)
	trans, _ := uni.GetTranslator("en")
	if err := tren.RegisterDefaultTranslations(v, trans); err != nil {
		panic(err)
	}
	if err := v.Struct(arg); err != nil {
		if vErr, ok := err.(validator.ValidationErrors); ok {
			vErr.Translate(trans)
			return errors.New(vErr.Error())
		}
		return err
	}
	return nil
}
