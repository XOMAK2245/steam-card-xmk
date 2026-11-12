// Package embedded carries the Steamworks worker files inside the executable
// and unpacks them on demand, so the application is a single self-contained file.
package embedded

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
)

// Extract writes the embedded worker files for the current OS into dir.
// Files whose content is already identical are left untouched (they may be
// loaded by a running worker).
func Extract(dir string) error {
	ents, err := files.ReadDir(sub)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		data, err := files.ReadFile(path.Join(sub, e.Name()))
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, e.Name())
		if old, err := os.ReadFile(dst); err == nil && bytes.Equal(old, data) {
			continue
		}
		tmp := dst + ".tmp"
		if err := os.WriteFile(tmp, data, 0o755); err != nil {
			return err
		}
		if err := os.Rename(tmp, dst); err != nil {
			return err
		}
	}
	return nil
}

// Has reports whether an embedded file with this name exists.
func Has(name string) bool {
	_, err := files.ReadFile(path.Join(sub, name))
	return err == nil
}
