// srs is a terminal UI for spaced-repetition flashcards.
//
// It exposes sub-commands for reviewing decks, creating cards, and
// managing configuration.  The real logic lives in internal/cli; this
// file is the minimal entry point that delegates to it.
package main

import (
	"os"

	"github.com/jvcorredor/srs-tui/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
