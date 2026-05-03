package main

import (
	"os"

	"github.com/jvcorredor/srs-tui/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
