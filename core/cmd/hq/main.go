// Command hq is the entry point of the personal setup.
//
// It does nothing itself: it hands the command tree to fang, which renders
// help, errors and --version. All behaviour lives in internal/cli and world.
package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/Gerc0g/dotfiles/core/internal/cli"
	"github.com/Gerc0g/dotfiles/core/runner"
	"github.com/charmbracelet/fang"
	"golang.org/x/term"
)

func main() {
	if handled, err := runner.SandboxCLI(context.Background(), os.Args[1:], os.Stdout); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := fang.Execute(
		context.Background(),
		cli.NewRoot(),
		fang.WithVersion(cli.Version),
		fang.WithErrorHandler(renderError),
	); err != nil {
		os.Exit(1)
	}
}

// Fang's prose formatter changes error casing. Machine clients depend on exact
// error codes, so only interactive terminals receive its presentation layer.
func renderError(w io.Writer, styles fang.Styles, err error) {
	if file, ok := w.(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		fang.DefaultErrorHandler(w, styles, err)
		return
	}
	fmt.Fprintln(w, err)
}
