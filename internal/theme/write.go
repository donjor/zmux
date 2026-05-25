package theme

import (
	"fmt"
	"strings"

	"github.com/donjor/zmux/internal/config"
)

// WriteFile serializes a Theme to a file in ghostty format.
func WriteFile(fs config.FS, path string, t Theme) error {
	content := Serialize(t)
	return fs.WriteFile(path, []byte(content), 0o644)
}

// Serialize converts a Theme to ghostty format string.
func Serialize(t Theme) string {
	var b strings.Builder

	fmt.Fprintf(&b, "background = %s\n", t.Background.Hex())
	fmt.Fprintf(&b, "foreground = %s\n", t.Foreground.Hex())
	fmt.Fprintf(&b, "cursor-color = %s\n", t.Cursor.Hex())

	fmt.Fprintf(&b, "selection-background = %s\n", t.Selection.Hex())

	for i := 0; i < 16; i++ {
		fmt.Fprintf(&b, "palette = %d=%s\n", i, t.Palette[i].Hex())
	}

	return b.String()
}
