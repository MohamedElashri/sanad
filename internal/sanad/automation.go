package sanad

import (
	"fmt"
	"strings"

	lock "github.com/github/actions-lockfile/go/pkg/lockfile"
)

func automation(root, ref string) error {
	if !lock.IsFullSha(ref) {
		return fmt.Errorf("automation requires --action-ref with the full commit of a released Sanad 1.x action")
	}
	check := `name: Sanad check
on:
  pull_request:
permissions:
  contents: read
jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1
        with:
          fetch-depth: 0
      - uses: MohamedElashri/sanad@ACTION_REF
        with:
          base: ${{ github.event.pull_request.base.sha }}
`
	update := `name: Sanad updates
on:
  schedule:
    - cron: '0 3 * * 1'
  workflow_dispatch:
permissions:
  contents: write
  pull-requests: write
jobs:
  update:
    uses: MohamedElashri/sanad/.github/workflows/update-pr.yml@ACTION_REF
    secrets:
      token: ${{ secrets.GITHUB_TOKEN }}
`
	for name, content := range map[string]string{".github/workflows/sanad-check.yml": check, ".github/workflows/sanad-update.yml": update} {
		// Never overwrite an independently maintained workflow.
		files, err := readTree(root, ConfigPath)
		if err != nil {
			return err
		}
		data := strings.ReplaceAll(content, "ACTION_REF", ref)
		if f, ok := files[name]; ok && string(f.Data) != data {
			return fmt.Errorf("%s already exists with different content", name)
		}
		if err := putFile(root, name, file{Data: []byte(data), Mode: 0o644}); err != nil {
			return err
		}
	}
	return nil
}
