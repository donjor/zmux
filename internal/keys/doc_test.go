package keys

import (
	"os"
	"path/filepath"
	"testing"
)

// TestKeybindingsDocInSync is the golden check: the committed
// docs/keybindings.md must match what GenerateDoc produces. If this fails, run
// `zmux keys gen` (or `make keys-gen`) and commit the result.
func TestKeybindingsDocInSync(t *testing.T) {
	want, err := GenerateDoc()
	if err != nil {
		t.Fatalf("GenerateDoc: %v", err)
	}
	path := filepath.Join("..", "..", "docs", "keybindings.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s is out of date — run `zmux keys gen` and commit the result", path)
	}
}
