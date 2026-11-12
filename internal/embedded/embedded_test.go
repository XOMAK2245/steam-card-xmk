package embedded

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExtract(t *testing.T) {
	dir := t.TempDir()
	if err := Extract(dir); err != nil {
		t.Fatal(err)
	}
	lib := map[string]string{"linux": "libsteam_api.so", "windows": "steam_api64.dll"}[runtime.GOOS]
	st, err := os.Stat(filepath.Join(dir, lib))
	if err != nil || st.Size() < 100000 {
		t.Fatalf("library not extracted: %v", err)
	}
	if err := Extract(dir); err != nil { // idempotent
		t.Fatal(err)
	}
}
