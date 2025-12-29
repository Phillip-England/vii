package vii

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EmbedByAbsolutePath is a convenience helper that:
//  1. serves local files from absDir at the given URL prefix
//  2. stores the same dir FS into the app's embedded registry under key == prefix
//
// Example:
//
//	app.EmbedByAbsolutePath("/assets", "/home/me/site/assets")
//
// Then:
//   - GET /assets/... serves from that directory
//   - EmbeddedDir(r, "/assets") and EmbeddedReadFile(r, "/assets", "file.txt") work
func (a *App) EmbedByAbsolutePath(prefix string, absDir string) error {
	if a == nil {
		return fmt.Errorf("vii: app is nil")
	}

	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return fmt.Errorf("vii: prefix is empty")
	}

	absDir = strings.TrimSpace(absDir)
	if absDir == "" {
		return fmt.Errorf("vii: absDir is empty")
	}

	// Ensure absolute + normalized path (and resolve symlinks for correctness).
	if !filepath.IsAbs(absDir) {
		p, err := filepath.Abs(absDir)
		if err != nil {
			return fmt.Errorf("vii: absDir resolve: %w", err)
		}
		absDir = p
	}
	if p, err := filepath.EvalSymlinks(absDir); err == nil {
		absDir = p
	}

	info, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("vii: stat absDir: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("vii: absDir is not a directory: %s", absDir)
	}

	// 1) Serve files at prefix
	if err := a.ServeLocalFiles(prefix, absDir); err != nil {
		return err
	}

	// 2) Also register it in embedded map so templates/helpers can read it.
	// Use the prefix itself as the key (simple + predictable).
	return a.EmbedDir(prefix, os.DirFS(absDir))
}
