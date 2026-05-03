// Package paths_test contains unit tests for XDG path resolution.
package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/paths"
)

func TestDataHomeUsesXDGDataHome(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "xdg-data")
	os.Setenv("XDG_DATA_HOME", custom)
	defer os.Unsetenv("XDG_DATA_HOME")

	got := paths.DataHome()
	want := custom
	if got != want {
		t.Errorf("DataHome() = %q, want %q", got, want)
	}
}

func TestDataHomeFallsBackToDefault(t *testing.T) {
	os.Unsetenv("XDG_DATA_HOME")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "share")

	got := paths.DataHome()
	if got != want {
		t.Errorf("DataHome() = %q, want %q", got, want)
	}
}

func TestDecksRootDefault(t *testing.T) {
	os.Unsetenv("XDG_DATA_HOME")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "share", "srs", "decks")

	got := paths.DecksRoot("")
	if got != want {
		t.Errorf("DecksRoot(\"\") = %q, want %q", got, want)
	}
}

func TestDecksRootOverride(t *testing.T) {
	got := paths.DecksRoot("/custom/path")
	want := "/custom/path"
	if got != want {
		t.Errorf("DecksRoot(\"/custom/path\") = %q, want %q", got, want)
	}
}

func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := paths.ExpandHome("~/my/decks")
	want := filepath.Join(home, "my/decks")
	if got != want {
		t.Errorf("ExpandHome(\"~/my/decks\") = %q, want %q", got, want)
	}
}

func TestExpandHomeNoTilde(t *testing.T) {
	got := paths.ExpandHome("/absolute/path")
	want := "/absolute/path"
	if got != want {
		t.Errorf("ExpandHome(\"/absolute/path\") = %q, want %q", got, want)
	}
}

func TestDecksRootExpandsTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := paths.DecksRoot("~/my-decks")
	want := filepath.Join(home, "my-decks")
	if got != want {
		t.Errorf("DecksRoot(\"~/my-decks\") = %q, want %q", got, want)
	}
}

func TestStateHomeUsesXDGStateHome(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "xdg-state")
	os.Setenv("XDG_STATE_HOME", custom)
	defer os.Unsetenv("XDG_STATE_HOME")

	got := paths.StateHome()
	if got != custom {
		t.Errorf("StateHome() = %q, want %q", got, custom)
	}
}

func TestConfigHomeUsesXDGConfigHome(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "xdg-config")
	os.Setenv("XDG_CONFIG_HOME", custom)
	defer os.Unsetenv("XDG_CONFIG_HOME")

	got := paths.ConfigHome()
	if got != custom {
		t.Errorf("ConfigHome() = %q, want %q", got, custom)
	}
}

func TestConfigHomeFallsBackToDefault(t *testing.T) {
	os.Unsetenv("XDG_CONFIG_HOME")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config")

	got := paths.ConfigHome()
	if got != want {
		t.Errorf("ConfigHome() = %q, want %q", got, want)
	}
}

func TestStateHomeFallsBackToDefault(t *testing.T) {
	os.Unsetenv("XDG_STATE_HOME")
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "state")

	got := paths.StateHome()
	if got != want {
		t.Errorf("StateHome() = %q, want %q", got, want)
	}
}
