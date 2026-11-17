package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func backupData(archiveName string) (result error) {
	archivePath, err := filepath.Abs(archiveName)
	if err != nil {
		return err
	}

	outFile, err := os.Create(archiveName)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() { result = errors.Join(result, outFile.Close()) }()

	gzWriter := gzip.NewWriter(outFile)
	defer func() { result = errors.Join(result, gzWriter.Close()) }()

	tarWriter := tar.NewWriter(gzWriter)
	defer func() { result = errors.Join(result, tarWriter.Close()) }()

	root := "."
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) (result error) {
		if err != nil {
			if os.IsPermission(err) {
				return nil // skip files/dirs with no permission
			}
			return err
		}

		pathAbs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if pathAbs == archivePath || (!info.IsDir() && strings.HasPrefix(info.Name(), "backup")) {
			return nil
		}

		var file *os.File
		if info.Mode().IsRegular() {
			file, err = os.Open(path)
			if err != nil {
				if os.IsPermission(err) {
					return nil // skip files with no permission
				}
				return err
			}
			defer func() { result = errors.Join(result, file.Close()) }()
		}

		hdr, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}

		// Set correct name in archive (relative path)
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil // skip root dir entry
		}
		hdr.Name = relPath

		// Try to preserve ownership (if running as root)
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			hdr.Uid = int(stat.Uid)
			hdr.Gid = int(stat.Gid)
		}

		if err := tarWriter.WriteHeader(hdr); err != nil {
			return err
		}

		if info.IsDir() {
			fmt.Printf("Backing up directory: %s\n", relPath)
		}
		if file != nil {
			if _, err := io.Copy(tarWriter, file); err != nil {
				return err
			}
		}

		return nil
	})
}
