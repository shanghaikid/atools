// Package installer handles copying embedded template files to the filesystem.
package installer

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Installer copies files from an embed.FS to the local filesystem.
type Installer struct {
	FS     fs.FS
	DryRun bool
}

// InstallFile copies a single embedded file to destPath.
// If the destination already has identical content, the write is skipped.
func (inst *Installer) InstallFile(embedPath, destPath string) error {
	data, err := fs.ReadFile(inst.FS, embedPath)
	if err != nil {
		return fmt.Errorf("read embedded %s: %w", embedPath, err)
	}

	// Skip if destination has identical content
	existing, err := os.ReadFile(destPath)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}

	if inst.DryRun {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("create dir for %s: %w", destPath, err)
	}
	return os.WriteFile(destPath, data, 0644)
}

// InstallDir copies all non-hidden files from an embedded directory to destDir.
// Subdirectories are skipped.
func (inst *Installer) InstallDir(embedDir, destDir string) error {
	entries, err := fs.ReadDir(inst.FS, embedDir)
	if err != nil {
		return fmt.Errorf("read embedded dir %s: %w", embedDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		src := embedDir + "/" + entry.Name()
		dst := filepath.Join(destDir, entry.Name())
		if err := inst.InstallFile(src, dst); err != nil {
			return err
		}
	}
	return nil
}
