package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func SetVersion(v, c, d string) {
	version = v
	commit = c
	date = d
}

func SetOutput(w io.Writer) {
	rootOut = w
}

var rootOut io.Writer

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "srs",
		Short: "Spaced repetition in the terminal",
	}
	root.SetOut(rootOut)

	root.AddCommand(newVersionCmd())
	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "srs %s\ncommit: %s\ndate:   %s\n", version, commit, date)
			return nil
		},
	}
}

func Execute() int {
	root := NewRootCmd()
	if err := root.Execute(); err != nil {
		return 1
	}
	return 0
}
