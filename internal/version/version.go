// Package version resolves the running binary's version, commit, and build
// date through three tiers: ldflags-injected vars, runtime/debug.BuildInfo
// (for go install @version), or sentinel defaults.
package version

import "runtime/debug"

// Version is the release version injected via ldflags at build time.
var Version string

// Commit is the Git commit hash injected via ldflags at build time.
var Commit string

// Date is the build timestamp injected via ldflags at build time.
var Date string

var readBuildInfo = debug.ReadBuildInfo

// Info holds the resolved version metadata.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	Source  string `json:"source"`
}

// Get resolves version metadata from ldflags, build info, or defaults.
func Get() Info {
	if Version != "" {
		return Info{Version: Version, Commit: Commit, Date: Date, Source: "ldflags"}
	}

	if bi, ok := readBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		info := Info{Version: bi.Main.Version, Commit: "unknown", Date: "unknown", Source: "buildinfo"}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				info.Commit = s.Value
			case "vcs.time":
				info.Date = s.Value
			}
		}
		return info
	}

	return Info{Version: "dev", Commit: "unknown", Date: "unknown", Source: "default"}
}

// SwapForTest replaces the package-level resolution inputs and returns a
// restore function. Test-only — never call from production code.
func SwapForTest(v, c, d string, rbi func() (*debug.BuildInfo, bool)) func() {
	prevV, prevC, prevD, prevRBI := Version, Commit, Date, readBuildInfo
	Version, Commit, Date = v, c, d
	readBuildInfo = rbi
	return func() {
		Version, Commit, Date = prevV, prevC, prevD
		readBuildInfo = prevRBI
	}
}
