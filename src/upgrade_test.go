package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewerVersion(t *testing.T) {
	for _, test := range []struct {
		current string
		latest  string
		want    bool
		wantErr bool
	}{
		{"0.4.0", "0.5.0", true, false},
		{"v0.4.0", "0.4.0", false, false},
		{"0.10.0", "0.9.0", false, false},
		{"1.0.0", "0.9.0", false, false},
		{"0.4.0", "0.4.1", true, false},
		{"0.4.0-rc.1", "0.4.0", true, false},
		{"0.4.0+build", "v0.4.0", false, false},
		{"dev", "0.4.0", true, false},
		{"dev", "", false, true},
		{"bad", "0.4.0", false, true},
		{"0.4.0", "0.5.0-rc.1", false, true},
		{"0.4.0", "0.x.0", false, true},
	} {
		t.Run(test.current+"->"+test.latest, func(t *testing.T) {
			got, err := newerVersion(test.current, test.latest)
			if got != test.want || (err != nil) != test.wantErr {
				t.Fatalf("newerVersion() = %v, %v; want %v, error=%v", got, err, test.want, test.wantErr)
			}
		})
	}
}

func TestUpgradeExecutable(t *testing.T) {
	const binary = "new executable"
	digest := sha256.Sum256([]byte(binary))
	for _, test := range []struct {
		name        string
		current     string
		answer      string
		releaseCode int
		assetCode   int
		body        string
		digest      string
		noAsset     bool
		message     string
		wantErr     string
		download    bool
		installed   bool
	}{
		{name: "confirmed", answer: "yes\n", download: true, installed: true, message: "Installed version"},
		{name: "short confirmation at EOF", answer: "Y", download: true, installed: true},
		{name: "cancelled", answer: "no\n", message: "Upgrade cancelled."},
		{name: "default no", message: "Upgrade cancelled."},
		{name: "same version", current: "0.5.0", message: "Already up to date."},
		{name: "no downgrade", current: "0.6.0", message: "Already up to date."},
		{name: "development build", current: "dev", answer: "y\n", download: true, installed: true},
		{name: "missing asset", noAsset: true, wantErr: "no usable infra binary"},
		{name: "API failure", releaseCode: http.StatusForbidden, wantErr: "query latest release: HTTP 403"},
		{name: "download failure", answer: "y\n", assetCode: http.StatusNotFound, download: true, wantErr: "download release: HTTP 404"},
		{name: "truncated download", answer: "y\n", body: "short", download: true, wantErr: "download size mismatch"},
		{name: "oversized download", answer: "y\n", body: binary + "extra", download: true, wantErr: "download size mismatch"},
		{name: "checksum failure", answer: "y\n", digest: "sha256:bad", download: true, wantErr: "checksum mismatch"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			executable := filepath.Join(dir, "infra")
			if err := os.WriteFile(executable, []byte("old executable"), 0751); err != nil {
				t.Fatal(err)
			}
			downloaded := false
			var server *httptest.Server
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/binary" {
					downloaded = true
					if test.assetCode != 0 {
						w.WriteHeader(test.assetCode)
						return
					}
					body := test.body
					if body == "" {
						body = binary
					}
					_, _ = w.Write([]byte(body))
					return
				}
				if test.releaseCode != 0 {
					w.WriteHeader(test.releaseCode)
					return
				}
				checksum := test.digest
				if checksum == "" {
					checksum = "sha256:" + hex.EncodeToString(digest[:])
				}
				release := githubRelease{Tag: "0.5.0"}
				if !test.noAsset {
					release.Assets = []releaseAsset{{
						Name: "infra", URL: server.URL + "/binary",
						Size: int64(len(binary)), Digest: checksum,
					}}
				}
				_ = json.NewEncoder(w).Encode(release)
			}))
			defer server.Close()
			current := test.current
			if current == "" {
				current = "0.4.0"
			}
			var output bytes.Buffer
			err := upgradeExecutable(server.Client(), server.URL, current, executable, strings.NewReader(test.answer), &output)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("error = %v, want %q", err, test.wantErr)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if test.releaseCode == 0 {
				for _, message := range []string{"Current version: " + current, "Latest version:  0.5.0", test.message} {
					if !strings.Contains(output.String(), message) {
						t.Errorf("output %q does not contain %q", output.String(), message)
					}
				}
			}
			// Closing waits for handlers before inspecting their download state.
			server.Close()
			if downloaded != test.download {
				t.Errorf("downloaded = %v, want %v", downloaded, test.download)
			}
			want := "old executable"
			if test.installed {
				want = binary
			}
			data, err := os.ReadFile(executable)
			if err != nil || string(data) != want {
				t.Errorf("executable = %q, %v; want %q", data, err, want)
			}
			info, err := os.Stat(executable)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0751 {
				t.Errorf("permissions = %o, want 751", info.Mode().Perm())
			}
			files, err := os.ReadDir(dir)
			if err != nil || len(files) != 1 {
				t.Errorf("temporary files not cleaned up: %v, %v", files, err)
			}
		})
	}
}
