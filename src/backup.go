package main

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pierrec/lz4/v4"
)

func backupData(archiveName string, excludes []string) (result error) {
	for _, pattern := range excludes {
		pattern = strings.TrimSuffix(pattern, "/")
		if pattern == "" {
			return fmt.Errorf("invalid empty backup exclude pattern")
		}
		if _, err := pathpkg.Match(pattern, ""); err != nil {
			return fmt.Errorf("invalid backup exclude pattern %q: %w", pattern, err)
		}
	}

	archivePath, err := filepath.Abs(archiveName)
	if err != nil {
		return err
	}

	outFile, err := os.Create(archiveName)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer func() { result = errors.Join(result, outFile.Close()) }()

	var compressedWriter io.WriteCloser
	if strings.EqualFold(filepath.Ext(archiveName), ".lz4") {
		compressedWriter = lz4.NewWriter(outFile)
	} else {
		compressedWriter = gzip.NewWriter(outFile)
	}
	defer func() { result = errors.Join(result, compressedWriter.Close()) }()

	tarWriter := tar.NewWriter(compressedWriter)
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
		if pathAbs == archivePath {
			return nil
		}

		relPath, err := filepath.Rel(".", path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)
		for _, pattern := range excludes {
			if relPath == "." {
				break
			}
			dirOnly := strings.HasSuffix(pattern, "/")
			matchPattern := strings.TrimSuffix(pattern, "/")
			if dirOnly && !info.IsDir() {
				continue
			}

			matched := false
			if strings.Contains(matchPattern, "/") {
				matched, err = pathpkg.Match(matchPattern, relPath)
				if err != nil {
					return fmt.Errorf("invalid backup exclude pattern %q: %w", pattern, err)
				}
			} else {
				for _, component := range strings.Split(relPath, "/") {
					matched, err = pathpkg.Match(matchPattern, component)
					if err != nil {
						return fmt.Errorf("invalid backup exclude pattern %q: %w", pattern, err)
					}
					if matched {
						break
					}
				}
			}
			if matched {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
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
