// Package config loads and provides application settings for srs-tui.
//
// Configuration is read from a TOML file located at
// <config-dir>/srs/config.toml, where <config-dir> defaults to
// $XDG_CONFIG_HOME or ~/.config.  When the file is missing, Load
// returns the built-in defaults.  All settings are optional; any
// key omitted from the file keeps its default value.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/jvcorredor/srs-tui/internal/paths"
)

// PathsConfig holds directory-path settings.
type PathsConfig struct {
	DecksRoot string `toml:"decks_root"`
}

// ReviewConfig holds review-session settings.
type ReviewConfig struct {
	NewPerDay int `toml:"new_per_day"`
}

// EditorConfig holds external-editor settings.
type EditorConfig struct {
	Command string `toml:"command"`
}

// RenderConfig holds TUI rendering settings.
type RenderConfig struct {
	Style string `toml:"style"`
}

// Config is the top-level configuration aggregate.
type Config struct {
	Paths  PathsConfig  `toml:"paths"`
	Review ReviewConfig `toml:"review"`
	Editor EditorConfig `toml:"editor"`
	Render RenderConfig `toml:"render"`
}

// Defaults returns a Config populated with built-in defaults.
func Defaults() *Config {
	return &Config{
		Paths: PathsConfig{
			DecksRoot: filepath.Join(paths.DataHome(), "srs", "decks"),
		},
		Review: ReviewConfig{
			NewPerDay: 20,
		},
		Editor: EditorConfig{
			Command: "",
		},
		Render: RenderConfig{
			Style: "dark",
		},
	}
}

// Load reads config.toml from <configDir>/srs and returns the merged
// result.  Missing files are treated as an empty config, so defaults
// are always preserved.  Tilde characters in DecksRoot are expanded
// to the user's home directory.
func Load(configDir string) (*Config, error) {
	cfg := Defaults()
	p := filepath.Join(configDir, "srs", "config.toml")

	if _, err := os.Stat(p); os.IsNotExist(err) {
		return cfg, nil
	}

	if _, err := toml.DecodeFile(p, cfg); err != nil {
		return nil, err
	}

	cfg.Paths.DecksRoot = paths.ExpandHome(cfg.Paths.DecksRoot)

	if cfg.Review.NewPerDay < 0 {
		return nil, fmt.Errorf("config: review.new_per_day must be non-negative, got %d", cfg.Review.NewPerDay)
	}

	return cfg, nil
}

// DefaultConfigContent returns the text embedded in a newly scaffolded
// config.toml file, including commented documentation for every section.
func DefaultConfigContent() string {
	return `# [paths]
# decks_root = ""    # Default: $XDG_DATA_HOME/srs/decks

# [review]
# new_per_day = 20   # New cards per day

# [editor]
# command = ""       # Editor for card creation (empty = $EDITOR or vi)

# [render]
# style = "dark"     # dark or light
`
}
