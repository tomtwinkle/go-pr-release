package cli_test

import (
	"strings"
	"testing"

	"github.com/bxcodec/faker/v3"
	"github.com/stretchr/testify/assert"

	"github.com/tomtwinkle/go-pr-release/internal/cli"
)

func TestLookupEnv(t *testing.T) {
	tests := map[string]struct {
		setEnv   func(t *testing.T)
		wantArgs *cli.Args
		wantErr  error
	}{
		"All": {
			setEnv: func(t *testing.T) {
				t.Setenv("GO_PR_RELEASE_DRY_RUN", "true")
				t.Setenv("GO_PR_RELEASE_TOKEN", "dummy")
				t.Setenv("GO_PR_RELEASE_TITLE", "title")
				t.Setenv("GO_PR_RELEASE_RELEASE", "main")
				t.Setenv("GO_PR_RELEASE_DEVELOP", "develop")
				t.Setenv("GO_PR_RELEASE_TEMPLATE", "./template.tmpl")
				t.Setenv("GO_PR_RELEASE_LABELS", "label1,label2")
				t.Setenv("GO_PR_RELEASE_REVIEWERS", "reviewer1,reviewer2")
			},
			wantArgs: &cli.Args{
				DryRun:        true,
				Token:         "dummy",
				Title:         "title",
				ReleaseBranch: "main",
				DevelopBranch: "develop",
				Template:      "./template.tmpl",
				Labels:        []string{"label1", "label2"},
				Reviewers:     []string{"reviewer1", "reviewer2"},
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			tt.setEnv(t)
			got, err := cli.LookupEnv()
			if tt.wantErr != nil {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantArgs, got)
		})
	}
}

func TestValidateArgs(t *testing.T) {
	tests := map[string]struct {
		args   *cli.Args
		errors []string
	}{
		"passed": {
			args: &cli.Args{
				Token:         faker.UUIDHyphenated(),
				ReleaseBranch: faker.UUIDDigit(),
				DevelopBranch: faker.UUIDDigit(),
			},
		},
		"required errors": {
			args: &cli.Args{},
			errors: []string{
				"--token or environment variable:GO_PR_RELEASE_TOKEN is required",
				"--release-branch or environment variable:GO_PR_RELEASE_RELEASE is required",
				"--develop-branch or environment variable:GO_PR_RELEASE_DEVELOP is required",
			},
		},
	}

	for n, v := range tests {
		name := n
		tt := v
		t.Run(name, func(t *testing.T) {
			err := cli.ValidateArgs(tt.args)
			if len(tt.errors) == 0 {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
			gotErrs := strings.Split(err.Error(), ",")
			if assert.Equal(t, len(tt.errors), len(gotErrs)) {
				for _, wantErr := range tt.errors {
					assert.Contains(t, err.Error(), wantErr)
				}
			}
		})
	}
}
