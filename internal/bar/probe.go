package bar

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Prober gathers VCS and language state for the status bar. It is an interface
// so the bar can be rendered without shelling out (in tests) and so these
// side-effects are injected rather than hardcoded, consistent with the rest of
// zmux (tmux.Runner, config.FS, …).
type Prober interface {
	GitBranch(dir string) string
	GitDirty(dir string) bool
	GitAheadBehind(dir string) (ahead, behind int)
	DetectLang(dir string) (icon, version string)
}

// ExecProber is the production Prober — it shells out to git and language
// toolchains. It is the default used by the bar-render command.
type ExecProber struct{}

var _ Prober = ExecProber{}

func (ExecProber) GitBranch(dir string) string {
	out, err := exec.Command("git", "-C", dir, "branch", "--show-current").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (ExecProber) GitDirty(dir string) bool {
	out, err := exec.Command("git", "-C", dir, "status", "--porcelain").Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func (ExecProber) GitAheadBehind(dir string) (ahead, behind int) {
	out, err := exec.Command("git", "-C", dir, "rev-list", "--left-right", "--count", "HEAD...@{upstream}").Output()
	if err != nil {
		return 0, 0
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 2 {
		_, _ = fmt.Sscanf(parts[0], "%d", &ahead)
		_, _ = fmt.Sscanf(parts[1], "%d", &behind)
	}
	return
}

func (ExecProber) DetectLang(dir string) (icon, version string) {
	if exists(filepath.Join(dir, "go.mod")) {
		out, err := exec.Command("go", "version").Output()
		if err == nil {
			parts := strings.Fields(string(out))
			if len(parts) >= 3 {
				return " ", strings.TrimPrefix(parts[2], "go")
			}
		}
	}
	if exists(filepath.Join(dir, "package.json")) {
		out, err := exec.Command("node", "-v").Output()
		if err == nil {
			return " ", strings.TrimSpace(strings.TrimPrefix(string(out), "v"))
		}
	}
	if exists(filepath.Join(dir, "Cargo.toml")) {
		out, err := exec.Command("rustc", "--version").Output()
		if err == nil {
			parts := strings.Fields(string(out))
			if len(parts) >= 2 {
				return " ", parts[1]
			}
		}
	}
	if exists(filepath.Join(dir, "requirements.txt")) || exists(filepath.Join(dir, "pyproject.toml")) {
		out, err := exec.Command("python3", "--version").Output()
		if err == nil {
			parts := strings.Fields(string(out))
			if len(parts) >= 2 {
				return " ", parts[1]
			}
		}
	}
	return "", ""
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
