package recipe

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

func Fork(fs config.FS, profile config.Profile, def Definition, force bool) (string, error) {
	if def.Source != SourceBundled {
		return "", fmt.Errorf("recipe %q is already a user recipe", def.Recipe.Name)
	}
	path, err := UserRecipePath(fs, profile, def.Recipe.Name)
	if err != nil {
		return "", err
	}
	if !force {
		if _, err := fs.Stat(path); err == nil {
			return "", fmt.Errorf("user recipe already exists: %s", path)
		}
	}
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := fs.WriteFile(path, def.Raw, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func CreateStarter(fs config.FS, profile config.Profile, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("recipe name is required")
	}
	path, err := UserRecipePath(fs, profile, name)
	if err != nil {
		return "", err
	}
	if _, err := fs.Stat(path); err == nil {
		return "", fmt.Errorf("user recipe already exists: %s", path)
	}
	r := Recipe{
		Name:        name,
		Description: "Local recipe",
		Kind:        KindSession,
		Workspace:   "{{ cwd_name | slug }}",
		Session:     "{{ workspace }}",
		Tabs: []TabSpec{
			{Name: "shell"},
		},
	}
	data, err := Marshal(r)
	if err != nil {
		return "", err
	}
	if err := fs.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, fs.WriteFile(path, data, 0o644)
}

func Edit(path string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	if len(parts) == 0 {
		return fmt.Errorf("empty editor command")
	}
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
