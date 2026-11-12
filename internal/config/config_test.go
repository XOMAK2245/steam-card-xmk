package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultAndLegacyMigration(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", tmp)
	t.Setenv("APPDATA", tmp)

	// 1. Fresh default
	s := Load()
	_, set := s.Snapshot()
	if set.Strategy != Batch || set.MaxParallel != 32 || set.PollMinutes != 5 {
		t.Fatalf("unexpected defaults: %+v", set)
	}

	// 2. Old config with "smart" and "parallelHours"
	cfgDir := Dir()
	os.MkdirAll(cfgDir, 0o700)
	legacy := `{"settings":{"strategy":"smart","maxParallel":30,"parallelHours":2,"pollMinutes":7}}`
	os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(legacy), 0o600)

	s2 := Load()
	_, set2 := s2.Snapshot()
	if set2.Strategy != Batch || set2.MaxParallel != 30 || set2.PollMinutes != 7 {
		t.Fatalf("failed to migrate smart config: %+v", set2)
	}

	// 3. Old config with "sequential"
	legacySeq := `{"settings":{"strategy":"sequential","maxParallel":0,"pollMinutes":2}}`
	os.WriteFile(filepath.Join(cfgDir, "config.json"), []byte(legacySeq), 0o600)

	s3 := Load()
	_, set3 := s3.Snapshot()
	if set3.Strategy != Single || set3.MaxParallel != 32 || set3.PollMinutes != 5 {
		t.Fatalf("failed to migrate sequential config: %+v", set3)
	}
}

func TestDiscordConfigPreserved(t *testing.T) {
	d := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", d)
	store := Load()
	store.Update(func(c *Store) {
		c.Discord.Token = "test_discord_token_xyz"
	})

	store2 := Load()
	if store2.Discord.Token != "test_discord_token_xyz" {
		t.Fatalf("Discord token not preserved: %q", store2.Discord.Token)
	}
}

