package theme

import "embed"

//go:embed bundled/*
var bundledFS embed.FS

// BundledFS returns the embedded filesystem containing bundled theme files.
// Files are at the path "bundled/<theme-name>".
func BundledFS() embed.FS {
	return bundledFS
}
