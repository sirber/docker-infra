package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateDefaultConfigUsesLZ4(t *testing.T) {
	path := filepath.Join(t.TempDir(), "infra.json")
	if err := createDefaultConfig(path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.ArchiveName != "backup.tar.lz4" {
		t.Errorf("default archive name = %q, want %q", cfg.ArchiveName, "backup.tar.lz4")
	}
}
