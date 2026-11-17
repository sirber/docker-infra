package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBackupData(t *testing.T) {
	root := t.TempDir()
	for _, dir := range []string{"archives", "service/nested", "backup-folder"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	for path, content := range map[string]string{
		"keep.txt":                 "root",
		"backup-old.tar.gz":        "old backup",
		"service/backup-data":      "excluded",
		"service/nested/keep.txt":  "nested",
		"backup-folder/keep.txt":   "directory retained",
		"service/Backup-upper.txt": "case sensitive",
	} {
		if err := os.WriteFile(filepath.Join(root, path), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Error(err)
		}
	})

	output, err := os.CreateTemp(t.TempDir(), "progress")
	if err != nil {
		t.Fatal(err)
	}
	oldStdout := os.Stdout
	os.Stdout = output
	err = backupData("archives/custom.tar.gz")
	os.Stdout = oldStdout
	if err != nil {
		t.Fatal(err)
	}
	if err := output.Close(); err != nil {
		t.Fatal(err)
	}
	progress, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	wantProgress := "Backing up directory: archives\nBacking up directory: backup-folder\nBacking up directory: service\nBacking up directory: service/nested\n"
	if string(progress) != wantProgress {
		t.Errorf("progress = %q, want %q", progress, wantProgress)
	}

	archive, err := os.Open("archives/custom.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	got := make(map[string]string)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		got[hdr.Name] = string(data)
	}
	want := map[string]string{
		"archives":                 "",
		"backup-folder":            "",
		"backup-folder/keep.txt":   "directory retained",
		"keep.txt":                 "root",
		"service":                  "",
		"service/Backup-upper.txt": "case sensitive",
		"service/nested":           "",
		"service/nested/keep.txt":  "nested",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("archive contents = %#v, want %#v", got, want)
	}
}

func TestBackupDataCreateError(t *testing.T) {
	err := backupData(filepath.Join(t.TempDir(), "missing", "backup.tar.gz"))
	if err == nil || !strings.Contains(err.Error(), "create archive") {
		t.Fatalf("expected archive creation error, got %v", err)
	}
}
