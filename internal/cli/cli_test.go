package cli_test

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/cli"
)

func TestVersionCommandPrintsVersion(t *testing.T) {
	buf := new(bytes.Buffer)
	cli.SetOutput(buf)
	cli.SetVersion("0.0.0-dev", "abc1234", "2026-01-01")

	cmd := cli.NewRootCmd()
	cmd.SetArgs([]string{"version"})
	cmd.SetOut(buf)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("version command failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0.0.0-dev") {
		t.Errorf("version output missing version string: got %q", out)
	}
	if !strings.Contains(out, "abc1234") {
		t.Errorf("version output missing commit: got %q", out)
	}
	if !strings.Contains(out, "2026-01-01") {
		t.Errorf("version output missing date: got %q", out)
	}
}

func TestExecuteReturnsZero(t *testing.T) {
	cli.SetOutput(io.Discard)
	code := cli.Execute()
	if code != 0 {
		t.Errorf("Execute() = %d, want 0", code)
	}
}
