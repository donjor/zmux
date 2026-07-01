package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentDoctrineKeepsPeerLoopStatusFirst(t *testing.T) {
	paths := []string{
		filepath.Join("..", "..", "skills", "zmux", "SKILL.md"),
		filepath.Join("..", "..", "skills", "zmux", "references", "agent-peer.md"),
		filepath.Join("..", "..", "skills", "zmux", "references", "agent-worker.md"),
	}
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		text := string(body)
		if !strings.Contains(text, "tab status") {
			t.Fatalf("%s must mention tab status as the read-side lifecycle API", path)
		}
		for _, stale := range []string{
			"Prefer `--idle` + classify for peer turns",
			"waiting for quiet screens with `watch --idle`",
			"Spawn, Wait (`watch --idle`)",
			"--idle 3 -T 300",
		} {
			if strings.Contains(text, stale) {
				t.Fatalf("%s still presents watch/idle as primary peer completion via %q", path, stale)
			}
		}
	}
}
