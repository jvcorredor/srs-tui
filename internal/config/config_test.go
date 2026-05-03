// Package config_test contains unit tests for the config loader.
package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/config"
)

// TestLoadReturnsDefaultsWhenFileAbsent confirms that Load returns the
// built-in defaults when no config.toml exists.
func TestLoadReturnsDefaultsWhenFileAbsent(t *testing.T) {
	cfg, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	defaults := config.Defaults()
	if cfg.Paths.DecksRoot != defaults.Paths.DecksRoot {
		t.Errorf("Paths.DecksRoot = %q, want %q", cfg.Paths.DecksRoot, defaults.Paths.DecksRoot)
	}
	if cfg.Review.NewPerDay != defaults.Review.NewPerDay {
		t.Errorf("Review.NewPerDay = %d, want %d", cfg.Review.NewPerDay, defaults.Review.NewPerDay)
	}
	if cfg.Editor.Command != defaults.Editor.Command {
		t.Errorf("Editor.Command = %q, want %q", cfg.Editor.Command, defaults.Editor.Command)
	}
	if cfg.Render.Style != defaults.Render.Style {
		t.Errorf("Render.Style = %q, want %q", cfg.Render.Style, defaults.Render.Style)
	}
}

// TestLoadParsesV1KeysFromConfigToml verifies that every documented config
// key is read correctly from a real TOML file.
func TestLoadParsesV1KeysFromConfigToml(t *testing.T) {
	dir := t.TempDir()
	srsDir := filepath.Join(dir, "srs")
	if err := os.MkdirAll(srsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `[paths]
decks_root = "/my/custom/decks"

[review]
new_per_day = 10

[editor]
command = "vim"

[render]
style = "light"
`
	if err := os.WriteFile(filepath.Join(srsDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Paths.DecksRoot != "/my/custom/decks" {
		t.Errorf("Paths.DecksRoot = %q, want %q", cfg.Paths.DecksRoot, "/my/custom/decks")
	}
	if cfg.Review.NewPerDay != 10 {
		t.Errorf("Review.NewPerDay = %d, want 10", cfg.Review.NewPerDay)
	}
	if cfg.Editor.Command != "vim" {
		t.Errorf("Editor.Command = %q, want %q", cfg.Editor.Command, "vim")
	}
	if cfg.Render.Style != "light" {
		t.Errorf("Render.Style = %q, want %q", cfg.Render.Style, "light")
	}
}

// TestLoadPartialConfigKeepsDefaults checks that omitted sections fall back
// to defaults rather than zero values.
func TestLoadPartialConfigKeepsDefaults(t *testing.T) {
	dir := t.TempDir()
	srsDir := filepath.Join(dir, "srs")
	if err := os.MkdirAll(srsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `[review]
new_per_day = 5
`
	if err := os.WriteFile(filepath.Join(srsDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	defaults := config.Defaults()
	if cfg.Paths.DecksRoot != defaults.Paths.DecksRoot {
		t.Errorf("Paths.DecksRoot = %q, want default %q", cfg.Paths.DecksRoot, defaults.Paths.DecksRoot)
	}
	if cfg.Review.NewPerDay != 5 {
		t.Errorf("Review.NewPerDay = %d, want 5", cfg.Review.NewPerDay)
	}
	if cfg.Render.Style != defaults.Render.Style {
		t.Errorf("Render.Style = %q, want default %q", cfg.Render.Style, defaults.Render.Style)
	}
}

// TestLoadExpandsTildeInDecksRoot ensures that a leading "~" in decks_root
// is expanded to the user's home directory.
func TestLoadExpandsTildeInDecksRoot(t *testing.T) {
	dir := t.TempDir()
	srsDir := filepath.Join(dir, "srs")
	if err := os.MkdirAll(srsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `[paths]
decks_root = "~/my-decks"
`
	if err := os.WriteFile(filepath.Join(srsDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "my-decks")
	if cfg.Paths.DecksRoot != want {
		t.Errorf("Paths.DecksRoot = %q, want %q", cfg.Paths.DecksRoot, want)
	}
}
