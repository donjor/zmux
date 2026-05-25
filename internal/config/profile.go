package config

import (
	"os"
	"path/filepath"
)

// Profile is the per-binary-name isolation profile. The live binary (`zmux`) and
// the edge binary (`zzmux`) resolve to different config/state/socket so the edge
// binary never collides with the live one — `zzmux apply`/`init`/workspace ops and
// source discovery all stay on the zzmux profile.
//
// Isolation is keyed purely off the invoking binary name (argv0): no env, no flags.
type Profile struct {
	Name         string // "zmux" | "zzmux"
	Socket       string // tmux -L value; "" means the default tmux server
	ConfigFile   string // ~/.zmux.toml | ~/.zzmux.toml
	ConfFile     string // generated tmux conf: ~/.tmux.conf | ~/.zzmux.conf
	StateDir     string // ~/.zmux | ~/.zzmux
	ThemesDir    string // <StateDir>/themes
	TemplatesDir string // <StateDir>/templates
	SnapshotsDir string // <StateDir>/snapshots
	DebugLog     string // <StateDir>/debug.log
}

// ProfileFromArgv resolves the active profile from the invoking binary name.
// argv0 is normally os.Args[0]; passing it explicitly keeps this testable without
// depending on the actual test-binary name. Uses the argv basename (not
// os.Executable, which resolves symlinks and would defeat name-based dispatch).
func ProfileFromArgv(argv0 string, fs FS) Profile {
	home, _ := fs.UserHomeDir()
	if filepath.Base(argv0) == "zzmux" {
		return profileFor(home, "zzmux", "zzmux", ".zzmux", ".zzmux.toml", ".zzmux.conf")
	}
	// Default zmux keeps its historical paths: state under ~/.zmux, but the
	// generated tmux conf is ~/.tmux.conf (not ~/.zmux.conf) for compatibility.
	return profileFor(home, "zmux", "", ".zmux", ".zmux.toml", ".tmux.conf")
}

func profileFor(home, name, socket, stateName, cfgName, confName string) Profile {
	dir := filepath.Join(home, stateName)
	return Profile{
		Name:         name,
		Socket:       socket,
		ConfigFile:   filepath.Join(home, cfgName),
		ConfFile:     filepath.Join(home, confName),
		StateDir:     dir,
		ThemesDir:    filepath.Join(dir, "themes"),
		TemplatesDir: filepath.Join(dir, "templates"),
		SnapshotsDir: filepath.Join(dir, "snapshots"),
		DebugLog:     filepath.Join(dir, "debug.log"),
	}
}

// ActiveProfile resolves the profile for the current process (from os.Args[0]).
// Leaf packages that compute paths without an injected profile use this; the
// composition root (internal/app) also resolves it once for the runner endpoint.
func ActiveProfile(fs FS) Profile {
	return ProfileFromArgv(os.Args[0], fs)
}
