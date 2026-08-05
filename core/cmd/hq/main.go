// Command hq is the entry point of the personal setup.
//
// It does nothing itself: it hands the command tree to fang, which renders
// help, errors and --version. All behaviour lives in internal/cli and world.
package main

import (
	"context"
	"os"

	"github.com/Gerc0g/dotfiles/core/internal/cli"
	"github.com/charmbracelet/fang"
)

func main() {
	if err := fang.Execute(
		context.Background(),
		cli.NewRoot(),
		fang.WithVersion(cli.Version),
	); err != nil {
		os.Exit(1)
	}
}
