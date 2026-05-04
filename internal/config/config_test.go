package config_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/config"
)

func TestLoadReturnsDefaultsWhenFileAbsent(t *testing.T) {
	cfg, warnings, err := config.Load(t.TempDir())
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
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want empty", warnings)
	}
}

func TestLoadDefaultRenderStyleIsAuto(t *testing.T) {
	cfg, _, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Render.Style != "auto" {
		t.Errorf("Render.Style = %q, want %q", cfg.Render.Style, "auto")
	}
}

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

	cfg, warnings, err := config.Load(dir)
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
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want empty", warnings)
	}
}

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

	cfg, _, err := config.Load(dir)
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

	cfg, _, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	home, _ := os.UserHomeDir()
	want := filepath.Join(home, "my-decks")
	if cfg.Paths.DecksRoot != want {
		t.Errorf("Paths.DecksRoot = %q, want %q", cfg.Paths.DecksRoot, want)
	}
}

func TestLoadRejectsNegativeNewPerDay(t *testing.T) {
	dir := t.TempDir()
	srsDir := filepath.Join(dir, "srs")
	if err := os.MkdirAll(srsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `[review]
new_per_day = -5
`
	if err := os.WriteFile(filepath.Join(srsDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load() should return error for negative new_per_day")
	}
}

func TestLoadTypeMismatchReturnsFieldError(t *testing.T) {
	dir := t.TempDir()
	srsDir := filepath.Join(dir, "srs")
	if err := os.MkdirAll(srsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `[review]
new_per_day = "not-a-number"
`
	if err := os.WriteFile(filepath.Join(srsDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := config.Load(dir)
	if err == nil {
		t.Fatal("Load() should return error for type mismatch")
	}

	var fe config.FieldError
	if !errors.As(err, &fe) {
		t.Fatalf("error should be FieldError, got %T: %v", err, err)
	}
	if fe.Key != "review.new_per_day" {
		t.Errorf("FieldError.Key = %q, want %q", fe.Key, "review.new_per_day")
	}
	if fe.File == "" {
		t.Error("FieldError.File should not be empty")
	}
	if fe.Reason == "" {
		t.Error("FieldError.Reason should not be empty")
	}
}

func TestLoadUnknownKeyProducesWarning(t *testing.T) {
	dir := t.TempDir()
	srsDir := filepath.Join(dir, "srs")
	if err := os.MkdirAll(srsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `[paths]
decks_root = "/valid"

[review]
new_per_day = 10

[typo]
some_key = "oops"
`
	if err := os.WriteFile(filepath.Join(srsDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, warnings, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config for unknown keys")
	}
	if cfg.Paths.DecksRoot != "/valid" {
		t.Errorf("Paths.DecksRoot = %q, want %q", cfg.Paths.DecksRoot, "/valid")
	}

	if len(warnings) == 0 {
		t.Fatal("Load() returned no warnings for unknown keys")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "typo.some_key") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("warnings = %v, want warning containing %q", warnings, "typo.some_key")
	}
}

func TestLoadInvalidRenderStyleFallsBackToAuto(t *testing.T) {
	dir := t.TempDir()
	srsDir := filepath.Join(dir, "srs")
	if err := os.MkdirAll(srsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	tomlContent := `[render]
style = "neon"
`
	if err := os.WriteFile(filepath.Join(srsDir, "config.toml"), []byte(tomlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, warnings, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Render.Style != "auto" {
		t.Errorf("Render.Style = %q, want %q (fallback)", cfg.Render.Style, "auto")
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "render.style") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("warnings = %v, want warning containing %q", warnings, "render.style")
	}
}
