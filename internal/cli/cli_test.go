package cli_test

import (
	"strings"
	"testing"

	"github.com/bxcodec/faker/v3"

	"github.com/stretchr/testify/assert"

	"github.com/tomtwinkle/go-pr-release/internal/cli"
)

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
				for i, gotErr := range gotErrs {
					assert.Contains(t, gotErr, tt.errors[i])
				}
			}
		})
	}
}
