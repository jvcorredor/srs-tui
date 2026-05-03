package integration_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func projectRoot(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}").Output()
	if err != nil {
		t.Fatalf("determine module root: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func goreleaserConfig(t *testing.T) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot(t), ".goreleaser.yml"))
	if err != nil {
		t.Fatalf("read .goreleaser.yml: %v", err)
	}
	return data
}

func TestGoreleaserConfigExistsAndIsValid(t *testing.T) {
	root := projectRoot(t)
	configPath := filepath.Join(root, ".goreleaser.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal(".goreleaser.yml does not exist")
	}

	cmd := exec.Command("goreleaser", "check", "-f", configPath)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goreleaser check failed:\n%s", out)
	}
}

func TestGoreleaserConfigHasFourBuildTargets(t *testing.T) {
	data := goreleaserConfig(t)

	var cfg struct {
		Builds []struct {
			Goos   []string `yaml:"goos"`
			Goarch []string `yaml:"goarch"`
		} `yaml:"builds"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}

	if len(cfg.Builds) == 0 {
		t.Fatal("no builds defined in .goreleaser.yml")
	}

	build := cfg.Builds[0]

	gotOS := map[string]bool{}
	for _, os := range build.Goos {
		gotOS[os] = true
	}
	if !gotOS["linux"] || !gotOS["darwin"] {
		t.Errorf("goos = %v, want linux and darwin", build.Goos)
	}

	gotArch := map[string]bool{}
	for _, arch := range build.Goarch {
		gotArch[arch] = true
	}
	if !gotArch["amd64"] || !gotArch["arm64"] {
		t.Errorf("goarch = %v, want amd64 and arm64", build.Goarch)
	}
}

func TestGoreleaserConfigHasLdflagsForVersionInjection(t *testing.T) {
	data := goreleaserConfig(t)

	var cfg struct {
		Builds []struct {
			Flags   []string `yaml:"flags"`
			Ldflags []string `yaml:"ldflags"`
			Env     []string `yaml:"env"`
		} `yaml:"builds"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}

	if len(cfg.Builds) == 0 {
		t.Fatal("no builds defined")
	}
	build := cfg.Builds[0]

	hasVersion := false
	hasCommit := false
	hasDate := false
	for _, ld := range build.Ldflags {
		if strings.Contains(ld, "version.Version") {
			hasVersion = true
		}
		if strings.Contains(ld, "version.Commit") {
			hasCommit = true
		}
		if strings.Contains(ld, "version.Date") {
			hasDate = true
		}
	}
	if !hasVersion {
		t.Error("ldflags missing version.Version injection")
	}
	if !hasCommit {
		t.Error("ldflags missing version.Commit injection")
	}
	if !hasDate {
		t.Error("ldflags missing version.Date injection")
	}

	hasCGODisabled := false
	for _, env := range build.Env {
		if env == "CGO_ENABLED=0" {
			hasCGODisabled = true
		}
	}
	if !hasCGODisabled {
		t.Error("env missing CGO_ENABLED=0")
	}

	hasTrimpath := false
	for _, flag := range build.Flags {
		if flag == "-trimpath" {
			hasTrimpath = true
		}
	}
	if !hasTrimpath {
		t.Error("flags missing -trimpath")
	}
}

func TestGoreleaserConfigHasArchiveAndChecksumSettings(t *testing.T) {
	data := goreleaserConfig(t)

	var cfg struct {
		Archives []struct {
			Formats      []string `yaml:"formats"`
			NameTemplate string   `yaml:"name_template"`
		} `yaml:"archives"`
		Checksum struct {
			NameTemplate string `yaml:"name_template"`
			Algorithm    string `yaml:"algorithm"`
		} `yaml:"checksum"`
		Release struct {
			Prerelease string `yaml:"prerelease"`
		} `yaml:"release"`
		Changelog struct {
			Disable bool `yaml:"disable"`
		} `yaml:"changelog"`
		Snapshot struct {
			VersionTemplate string `yaml:"version_template"`
		} `yaml:"snapshot"`
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yml: %v", err)
	}

	if len(cfg.Archives) == 0 {
		t.Fatal("no archives defined")
	}
	archive := cfg.Archives[0]
	hasTarGz := false
	for _, f := range archive.Formats {
		if f == "tar.gz" {
			hasTarGz = true
		}
	}
	if !hasTarGz {
		t.Errorf("archives.formats = %v, want tar.gz", archive.Formats)
	}
	if cfg.Checksum.Algorithm != "sha256" {
		t.Errorf("checksum.algorithm = %q, want %q", cfg.Checksum.Algorithm, "sha256")
	}
	if cfg.Release.Prerelease != "auto" {
		t.Errorf("release.prerelease = %q, want %q", cfg.Release.Prerelease, "auto")
	}
	if !cfg.Changelog.Disable {
		t.Error("changelog.disable = false, want true")
	}
	if cfg.Snapshot.VersionTemplate != "{{ incpatch .Version }}-snapshot" {
		t.Errorf("snapshot.version_template = %q, want %q", cfg.Snapshot.VersionTemplate, "{{ incpatch .Version }}-snapshot")
	}
}

func runSnapshotBuild(t *testing.T) string {
	t.Helper()
	root := projectRoot(t)
	distDir := filepath.Join(root, "dist")

	cmd := exec.Command("goreleaser", "release", "--snapshot", "--clean")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("goreleaser release --snapshot --clean failed:\n%s", out)
	}

	return distDir
}

func TestSnapshotBuildProducesFourArchives(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping snapshot build in short mode")
	}

	distDir := runSnapshotBuild(t)

	expectedArchives := []string{
		"darwin_amd64",
		"darwin_arm64",
		"linux_amd64",
		"linux_arm64",
	}

	entries, err := os.ReadDir(distDir)
	if err != nil {
		t.Fatalf("read dist dir: %v", err)
	}

	found := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		for _, target := range expectedArchives {
			if strings.Contains(name, target) && strings.HasSuffix(name, ".tar.gz") {
				found[target] = true
			}
		}
	}

	for _, target := range expectedArchives {
		if !found[target] {
			t.Errorf("missing archive for %s", target)
		}
	}
}

func TestSnapshotBuildBinaryReportsVersionViaLdflags(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping snapshot build in short mode")
	}

	distDir := runSnapshotBuild(t)

	goos := runtime.GOOS
	goarch := runtime.GOARCH

	var binaryPath string
	entries, err := os.ReadDir(distDir)
	if err != nil {
		t.Fatalf("read dist dir: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() && strings.Contains(e.Name(), goos) && strings.Contains(e.Name(), goarch) {
			candidates, _ := os.ReadDir(filepath.Join(distDir, e.Name()))
			for _, c := range candidates {
				if !c.IsDir() && !strings.HasSuffix(c.Name(), ".tar.gz") {
					binaryPath = filepath.Join(distDir, e.Name(), c.Name())
					break
				}
			}
		}
	}
	if binaryPath == "" {
		t.Fatalf("could not find native binary for %s/%s in dist/", goos, goarch)
	}

	cmd := exec.Command(binaryPath, "version", "--format=json")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s version --format=json: %v\n%s", binaryPath, err, out)
	}

	var info struct {
		Version string `json:"version"`
		Source  string `json:"source"`
	}
	if err := json.Unmarshal(out, &info); err != nil {
		t.Fatalf("parse version json: %v\noutput: %s", err, out)
	}

	if info.Source != "ldflags" {
		t.Errorf("version source = %q, want %q (ldflags injection failed)", info.Source, "ldflags")
	}
	if !strings.HasSuffix(info.Version, "-snapshot") {
		t.Errorf("version = %q, want suffix -snapshot", info.Version)
	}
}

func TestSnapshotBuildGeneratesChecksums(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping snapshot build in short mode")
	}

	distDir := runSnapshotBuild(t)

	checksumsPath := filepath.Join(distDir, "checksums.txt")
	if _, err := os.Stat(checksumsPath); os.IsNotExist(err) {
		t.Fatal("checksums.txt not found in dist/")
	}

	data, err := os.ReadFile(checksumsPath)
	if err != nil {
		t.Fatalf("read checksums.txt: %v", err)
	}

	content := string(data)
	if len(content) == 0 {
		t.Error("checksums.txt is empty")
	}

	lines := strings.Count(content, "\n")
	if lines < 4 {
		t.Errorf("checksums.txt has %d lines, want at least 4 (one per archive)", lines)
	}
}

func TestJustfileHasReleaseSnapshotRecipe(t *testing.T) {
	root := projectRoot(t)

	cmd := exec.Command("just", "--list")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("just --list: %v", err)
	}

	list := string(out)
	if !strings.Contains(list, "release-snapshot") {
		t.Errorf("just --list does not contain 'release-snapshot', got:\n%s", list)
	}
}

func TestJustReleaseSnapshotProducesBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping snapshot build in short mode")
	}

	root := projectRoot(t)
	distDir := filepath.Join(root, "dist")

	cmd := exec.Command("just", "release-snapshot")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("just release-snapshot failed:\n%s", out)
	}

	if _, err := os.Stat(distDir); os.IsNotExist(err) {
		t.Fatal("dist/ directory not created by just release-snapshot")
	}

	checksumsPath := filepath.Join(distDir, "checksums.txt")
	if _, err := os.Stat(checksumsPath); os.IsNotExist(err) {
		t.Error("checksums.txt not found in dist/ after just release-snapshot")
	}
}
