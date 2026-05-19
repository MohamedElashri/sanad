package main

import (
	"fmt"
	"io"
	"os"

	"github.com/MohamedElashri/sanad/internal/cli"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	root := cli.NewRootCommand()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	if err := root.Execute(); err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return cli.ExitCode(err)
	}
	return cli.ExitCode(nil)
}
