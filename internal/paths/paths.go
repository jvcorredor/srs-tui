// Package paths resolves directories according to the XDG Base Directory
// specification, providing sensible fallbacks when the environment variables
// are unset.
package paths

import (
	"os"
	"path/filepath"
)

// ConfigHome returns the XDG configuration directory ($XDG_CONFIG_HOME,
// or ~/.config by default).
func ConfigHome() string {
	if v := os.Getenv("XDG_CONFIG_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}

// DataHome returns the XDG data directory ($XDG_DATA_HOME, or
// ~/.local/share by default).
func DataHome() string {
	if v := os.Getenv("XDG_DATA_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share")
}

// StateHome returns the XDG state directory ($XDG_STATE_HOME, or
// ~/.local/state by default).
func StateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}

// DecksRoot returns the root directory for deck storage.  If override is
// non-empty it is used verbatim (with tilde expansion); otherwise the
// default $XDG_DATA_HOME/srs/decks is returned.
func DecksRoot(override string) string {
	if override != "" {
		return ExpandHome(override)
	}
	return filepath.Join(DataHome(), "srs", "decks")
}

// ExpandHome replaces a leading "~" in p with the user's home directory.
func ExpandHome(p string) string {
	if len(p) > 0 && p[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, p[1:])
	}
	return p
}
