package main

import (
	"os"

	"github.com/tomtwinkle/go-pr-release/internal/cli"
)

var (
	name    string
	version string
	commit  string
	date    string
)

func main() {
	os.Exit(cli.Run(name, version, commit, date))
}
