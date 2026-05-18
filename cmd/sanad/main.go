package main

import (
	"os"

	"github.com/MohamedElashri/sanad/internal/cli"
)

func main() {
	root := cli.NewRootCommand()
	if err := root.Execute(); err != nil {
		os.Exit(cli.ExitCode(err))
	}
}
