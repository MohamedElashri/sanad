package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/MohamedElashri/sanad/internal/sanad"
)

func main() {
	native := flag.Bool("native", false, "download only the current platform")
	output := flag.String("output", "", "destination file for --native")
	flag.Parse()
	targets := sanad.BackendTargets()
	if *native {
		target := runtime.GOOS + "-" + runtime.GOARCH
		if runtime.GOOS == "windows" {
			target += ".exe"
		}
		targets = []string{target}
	}
	if *output != "" && !*native {
		fmt.Fprintln(os.Stderr, "--output requires --native")
		os.Exit(2)
	}
	for _, asset := range targets {
		target := strings.TrimSuffix(asset, ".exe")
		parts := strings.Split(target, "-")
		name := "gh-actions-lock"
		if strings.HasSuffix(asset, ".exe") {
			name += ".exe"
		}
		destination := filepath.Join(".release-tools", parts[0]+"_"+parts[1], name)
		if *output != "" {
			destination = *output
		}
		if err := sanad.FetchBackend(context.Background(), asset, destination); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(destination)
	}
}
