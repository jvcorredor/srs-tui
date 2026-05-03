package version_test

import (
	"encoding/json"
	"runtime/debug"
	"testing"

	"github.com/jvcorredor/srs-tui/internal/version"
)

func TestGetReturnsSentinelDefaultsWhenNothingResolved(t *testing.T) {
	defer version.SwapForTest("", "", "", func() (*debug.BuildInfo, bool) { return nil, false })()

	got := version.Get()

	if got.Version != "dev" {
		t.Errorf("Version = %q, want %q", got.Version, "dev")
	}
	if got.Commit != "unknown" {
		t.Errorf("Commit = %q, want %q", got.Commit, "unknown")
	}
	if got.Date != "unknown" {
		t.Errorf("Date = %q, want %q", got.Date, "unknown")
	}
	if got.Source != "default" {
		t.Errorf("Source = %q, want %q", got.Source, "default")
	}
}

func TestGetUsesLdflagsValuesWhenSet(t *testing.T) {
	defer version.SwapForTest("v1.2.3", "abc1234", "2026-05-03T12:00:00Z", func() (*debug.BuildInfo, bool) {
		t.Fatal("readBuildInfo should not be consulted when ldflags vars are set")
		return nil, false
	})()

	got := version.Get()

	if got.Version != "v1.2.3" {
		t.Errorf("Version = %q, want %q", got.Version, "v1.2.3")
	}
	if got.Commit != "abc1234" {
		t.Errorf("Commit = %q, want %q", got.Commit, "abc1234")
	}
	if got.Date != "2026-05-03T12:00:00Z" {
		t.Errorf("Date = %q, want %q", got.Date, "2026-05-03T12:00:00Z")
	}
	if got.Source != "ldflags" {
		t.Errorf("Source = %q, want %q", got.Source, "ldflags")
	}
}

func TestGetReadsBuildInfoWhenLdflagsEmpty(t *testing.T) {
	fixture := &debug.BuildInfo{
		Main: debug.Module{Version: "v0.5.1"},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "deadbeefcafe"},
			{Key: "vcs.time", Value: "2026-04-01T08:30:00Z"},
		},
	}
	defer version.SwapForTest("", "", "", func() (*debug.BuildInfo, bool) { return fixture, true })()

	got := version.Get()

	if got.Version != "v0.5.1" {
		t.Errorf("Version = %q, want %q", got.Version, "v0.5.1")
	}
	if got.Commit != "deadbeefcafe" {
		t.Errorf("Commit = %q, want %q", got.Commit, "deadbeefcafe")
	}
	if got.Date != "2026-04-01T08:30:00Z" {
		t.Errorf("Date = %q, want %q", got.Date, "2026-04-01T08:30:00Z")
	}
	if got.Source != "buildinfo" {
		t.Errorf("Source = %q, want %q", got.Source, "buildinfo")
	}
}

func TestGetUsesDefaultsWhenBuildInfoMainVersionDevel(t *testing.T) {
	fixture := &debug.BuildInfo{
		Main: debug.Module{Version: "(devel)"},
	}
	defer version.SwapForTest("", "", "", func() (*debug.BuildInfo, bool) { return fixture, true })()

	got := version.Get()

	if got.Source != "default" {
		t.Errorf("Source = %q, want %q (Main.Version=(devel) should fall through)", got.Source, "default")
	}
	if got.Version != "dev" {
		t.Errorf("Version = %q, want %q", got.Version, "dev")
	}
}

func TestInfoJSONRoundTrip(t *testing.T) {
	original := version.Info{
		Version: "v0.2.0",
		Commit:  "1234abcd",
		Date:    "2026-05-03T12:00:00Z",
		Source:  "ldflags",
	}

	raw, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var asMap map[string]string
	if err := json.Unmarshal(raw, &asMap); err != nil {
		t.Fatalf("Unmarshal to map: %v", err)
	}
	for _, key := range []string{"version", "commit", "date", "source"} {
		if _, ok := asMap[key]; !ok {
			t.Errorf("JSON missing %q key, got: %s", key, raw)
		}
	}

	var roundTripped version.Info
	if err := json.Unmarshal(raw, &roundTripped); err != nil {
		t.Fatalf("Unmarshal to Info: %v", err)
	}
	if roundTripped != original {
		t.Errorf("round trip mismatch: got %+v, want %+v", roundTripped, original)
	}
}
