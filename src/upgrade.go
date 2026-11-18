package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const latestReleaseURL = "https://api.github.com/repos/sirber/docker-infra/releases/latest"

type releaseAsset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Size   int64  `json:"size"`
	Digest string `json:"digest"`
}

type githubRelease struct {
	Tag    string         `json:"tag_name"`
	Assets []releaseAsset `json:"assets"`
}

func upgrade() error {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return fmt.Errorf("self-upgrade is only supported on linux/amd64")
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	return upgradeExecutable(client, latestReleaseURL, Version, executable, os.Stdin, os.Stdout)
}

func upgradeExecutable(client *http.Client, releaseURL, current, executable string, input io.Reader, output io.Writer) error {
	response, err := client.Get(releaseURL)
	if err != nil {
		return fmt.Errorf("query latest release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("query latest release: HTTP %s", response.Status)
	}
	var release githubRelease
	if err := json.NewDecoder(response.Body).Decode(&release); err != nil {
		return fmt.Errorf("decode latest release: %w", err)
	}
	fmt.Fprintf(output, "Current version: %s\nLatest version:  %s\n", current, release.Tag)
	newer, err := newerVersion(current, release.Tag)
	if err != nil {
		return err
	}
	if !newer {
		fmt.Fprintln(output, "Already up to date.")
		return nil
	}
	var asset releaseAsset
	for _, candidate := range release.Assets {
		if candidate.Name == "infra" {
			asset = candidate
			break
		}
	}
	if asset.URL == "" || asset.Size <= 0 {
		return fmt.Errorf("release %s has no usable infra binary", release.Tag)
	}
	fmt.Fprint(output, "Download and install this release? [y/N]: ")
	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && err != io.EOF {
		return fmt.Errorf("read confirmation: %w", err)
	}
	answer = strings.TrimSpace(answer)
	if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
		fmt.Fprintln(output, "Upgrade cancelled.")
		return nil
	}
	if err := installRelease(client, asset, executable); err != nil {
		return err
	}
	fmt.Fprintf(output, "Installed version %s. Run infra again to use it.\n", release.Tag)
	return nil
}

// GitHub's latest endpoint excludes prereleases; compare stable release numbers,
// treating a local prerelease as older than the same stable version.
func newerVersion(current, latest string) (bool, error) {
	parse := func(version string) ([3]uint64, bool, error) {
		var numbers [3]uint64
		base, _, _ := strings.Cut(strings.TrimPrefix(version, "v"), "+")
		base, prerelease, _ := strings.Cut(base, "-")
		parts := strings.Split(base, ".")
		if len(parts) != len(numbers) {
			return numbers, false, fmt.Errorf("unsupported version %q: expected major.minor.patch", version)
		}
		for i, part := range parts {
			number, err := strconv.ParseUint(part, 10, 64)
			if err != nil {
				return numbers, false, fmt.Errorf("invalid version %q: %w", version, err)
			}
			numbers[i] = number
		}
		return numbers, prerelease != "", nil
	}
	remote, prerelease, err := parse(latest)
	if err != nil {
		return false, err
	}
	if prerelease {
		return false, fmt.Errorf("latest release %q is not a stable version", latest)
	}
	if current == "dev" {
		return true, nil
	}
	local, localPrerelease, err := parse(current)
	if err != nil {
		return false, err
	}
	for i := range remote {
		if remote[i] != local[i] {
			return remote[i] > local[i], nil
		}
	}
	return localPrerelease, nil
}

func installRelease(client *http.Client, asset releaseAsset, executable string) error {
	info, err := os.Stat(executable)
	if err != nil {
		return fmt.Errorf("inspect installed executable: %w", err)
	}
	// Stage beside the executable so rename replaces it atomically, even while running.
	file, err := os.CreateTemp(filepath.Dir(executable), ".infra-upgrade-*")
	if err != nil {
		return fmt.Errorf("create replacement (check installation directory permissions): %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	response, err := client.Get(asset.URL)
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download release: HTTP %s", response.Status)
	}
	hash := sha256.New()
	written, err := io.Copy(io.MultiWriter(file, hash), io.LimitReader(response.Body, asset.Size+1))
	if err != nil {
		return fmt.Errorf("download release: %w", err)
	}
	if written != asset.Size {
		return fmt.Errorf("download size mismatch: got %d bytes, expected %d", written, asset.Size)
	}
	if asset.Digest != "" && asset.Digest != "sha256:"+hex.EncodeToString(hash.Sum(nil)) {
		return fmt.Errorf("download checksum mismatch")
	}
	if err := file.Chmod(info.Mode().Perm()); err != nil {
		return fmt.Errorf("set executable permissions: %w", err)
	}
	if err := file.Sync(); err != nil {
		return fmt.Errorf("sync replacement: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close replacement: %w", err)
	}
	if err := os.Rename(file.Name(), executable); err != nil {
		return fmt.Errorf("replace installed executable: %w", err)
	}
	return nil
}
