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
	"strings"

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

// FieldError represents a validation error for a specific config key.
type FieldError struct {
	File   string
	Key    string
	Reason string
}

func (e FieldError) Error() string {
	return fmt.Sprintf("%s: %s: %s", e.File, e.Key, e.Reason)
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
			Style: "auto",
		},
	}
}

// knownKeys lists every valid dotted key in the v1 config surface.
var knownKeys = map[string]bool{
	"paths":              true,
	"paths.decks_root":   true,
	"review":             true,
	"review.new_per_day": true,
	"editor":             true,
	"editor.command":     true,
	"render":             true,
	"render.style":       true,
}

var glamourStyles = map[string]bool{
	"auto":  true,
	"dark":  true,
	"light": true,
	"notty": true,
	"pink":  true,
}

// Load reads config.toml from <configDir>/srs and returns the merged
// result along with any warnings for unknown keys.  Missing files are
// treated as an empty config, so defaults are always preserved.  Tilde
// characters in DecksRoot are expanded to the user's home directory.
func Load(configDir string) (*Config, []string, error) {
	cfg := Defaults()
	p := filepath.Join(configDir, "srs", "config.toml")

	if _, err := os.Stat(p); os.IsNotExist(err) {
		return cfg, nil, nil
	}

	meta, err := toml.DecodeFile(p, cfg)
	if err != nil {
		return nil, nil, wrapDecodeError(err, p)
	}

	var warnings []string
	for _, key := range meta.Keys() {
		dotted := strings.Join(key, ".")
		if !knownKeys[dotted] {
			warnings = append(warnings, fmt.Sprintf("warning: unknown config key %q in %s", dotted, p))
		}
	}

	cfg.Paths.DecksRoot = paths.ExpandHome(cfg.Paths.DecksRoot)

	if !glamourStyles[cfg.Render.Style] {
		warnings = append(warnings, fmt.Sprintf("warning: unknown render.style %q in %s; falling back to %q", cfg.Render.Style, p, "auto"))
		cfg.Render.Style = "auto"
	}

	if cfg.Review.NewPerDay < 0 {
		return nil, warnings, FieldError{
			File:   p,
			Key:    "review.new_per_day",
			Reason: fmt.Sprintf("must be non-negative, got %d", cfg.Review.NewPerDay),
		}
	}

	return cfg, warnings, nil
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
# style = "auto"     # auto, dark, light, notty, pink
`
}

func wrapDecodeError(err error, file string) error {
	s := err.Error()
	prefix := `(last key "`
	i := strings.Index(s, prefix)
	if i < 0 {
		return err
	}
	i += len(prefix)
	j := strings.Index(s[i:], `"`)
	if j < 0 {
		return err
	}
	key := s[i : i+j]
	reasonStart := strings.Index(s[i+j:], "): ")
	if reasonStart < 0 {
		return err
	}
	reason := s[i+j+reasonStart+3:]
	return FieldError{File: file, Key: key, Reason: reason}
}
