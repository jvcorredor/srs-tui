// Package config loads and provides default application configuration.
package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/jvcorredor/srs-tui/internal/paths"
)

type PathsConfig struct {
	DecksRoot string `toml:"decks_root"`
}

type ReviewConfig struct {
	NewPerDay int `toml:"new_per_day"`
}

type EditorConfig struct {
	Command string `toml:"command"`
}

type RenderConfig struct {
	Style string `toml:"style"`
}

type Config struct {
	Paths  PathsConfig  `toml:"paths"`
	Review ReviewConfig `toml:"review"`
	Editor EditorConfig `toml:"editor"`
	Render RenderConfig `toml:"render"`
}

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

	return cfg, nil
}

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
