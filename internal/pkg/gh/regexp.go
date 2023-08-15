package gh

import "regexp"

var RegRefPullRequest = regexp.MustCompile(`^refs/pull/(\d+)/head$`)
