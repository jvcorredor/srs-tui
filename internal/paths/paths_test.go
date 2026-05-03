// Package paths_test contains unit tests for XDG path resolution.
package paths_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/paths"
)

// TestDataHomeUsesXDGDataHome checks that DataHome respects $XDG_DATA_HOME.
func TestDataHomeUsesXDGDataHome(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "xdg-data")
	if err := os.Setenv("XDG_DATA_HOME", custom); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_DATA_HOME"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
	}()

	got := paths.DataHome()
	want := custom
	if got != want {
		t.Errorf("DataHome() = %q, want %q", got, want)
	}
}

// TestDataHomeFallsBackToDefault verifies the default ~/.local/share fallback.
func TestDataHomeFallsBackToDefault(t *testing.T) {
	if err := os.Unsetenv("XDG_DATA_HOME"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "share")

	got := paths.DataHome()
	if got != want {
		t.Errorf("DataHome() = %q, want %q", got, want)
	}
}

// TestDecksRootDefault confirms DecksRoot("") returns the standard SRS decks path.
func TestDecksRootDefault(t *testing.T) {
	if err := os.Unsetenv("XDG_DATA_HOME"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "share", "srs", "decks")

	got := paths.DecksRoot("")
	if got != want {
		t.Errorf("DecksRoot(\"\") = %q, want %q", got, want)
	}
}

// TestDecksRootOverride checks that a non-empty override is passed through verbatim.
func TestDecksRootOverride(t *testing.T) {
	got := paths.DecksRoot("/custom/path")
	want := "/custom/path"
	if got != want {
		t.Errorf("DecksRoot(\"/custom/path\") = %q, want %q", got, want)
	}
}

// TestExpandHome verifies tilde expansion to the user's home directory.
func TestExpandHome(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := paths.ExpandHome("~/my/decks")
	want := filepath.Join(home, "my/decks")
	if got != want {
		t.Errorf("ExpandHome(\"~/my/decks\") = %q, want %q", got, want)
	}
}

// TestExpandHomeNoTilde ensures that absolute paths without a tilde are left untouched.
func TestExpandHomeNoTilde(t *testing.T) {
	got := paths.ExpandHome("/absolute/path")
	want := "/absolute/path"
	if got != want {
		t.Errorf("ExpandHome(\"/absolute/path\") = %q, want %q", got, want)
	}
}

// TestDecksRootExpandsTilde confirms that DecksRoot expands a leading "~".
func TestDecksRootExpandsTilde(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := paths.DecksRoot("~/my-decks")
	want := filepath.Join(home, "my-decks")
	if got != want {
		t.Errorf("DecksRoot(\"~/my-decks\") = %q, want %q", got, want)
	}
}

// TestStateHomeUsesXDGStateHome checks that StateHome respects $XDG_STATE_HOME.
func TestStateHomeUsesXDGStateHome(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "xdg-state")
	if err := os.Setenv("XDG_STATE_HOME", custom); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_STATE_HOME"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
	}()

	got := paths.StateHome()
	if got != custom {
		t.Errorf("StateHome() = %q, want %q", got, custom)
	}
}

// TestConfigHomeUsesXDGConfigHome checks that ConfigHome respects $XDG_CONFIG_HOME.
func TestConfigHomeUsesXDGConfigHome(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "xdg-config")
	if err := os.Setenv("XDG_CONFIG_HOME", custom); err != nil {
		t.Fatalf("setenv: %v", err)
	}
	defer func() {
		if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
			t.Fatalf("unsetenv: %v", err)
		}
	}()

	got := paths.ConfigHome()
	if got != custom {
		t.Errorf("ConfigHome() = %q, want %q", got, custom)
	}
}

// TestConfigHomeFallsBackToDefault verifies the default ~/.config fallback.
func TestConfigHomeFallsBackToDefault(t *testing.T) {
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config")

	got := paths.ConfigHome()
	if got != want {
		t.Errorf("ConfigHome() = %q, want %q", got, want)
	}
}

// TestStateHomeFallsBackToDefault verifies the default ~/.local/state fallback.
func TestStateHomeFallsBackToDefault(t *testing.T) {
	if err := os.Unsetenv("XDG_STATE_HOME"); err != nil {
		t.Fatalf("unsetenv: %v", err)
	}
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".local", "state")

	got := paths.StateHome()
	if got != want {
		t.Errorf("StateHome() = %q, want %q", got, want)
	}
}
