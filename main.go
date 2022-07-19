package main

import (
	"os"

	"github.com/tomtwinkle/go-pr-release/internal/cli"
)

var Name string
var Version string

func main() {
	os.Exit(cli.Run(Name, Version))
}
