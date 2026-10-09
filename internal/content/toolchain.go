package content

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// The official metadata supplies the archive checksum. Compilation remains a
// manual action; downloaded code is never selected by joining a game server.
func nativeToolchain(root, goMod string) (string, error) {
	if path, err := exec.LookPath("go"); err == nil {
		return path, nil
	}
	if runtime.GOOS != "windows" || runtime.GOARCH != "amd64" {
		return "", fmt.Errorf("install Go before building native modules on this platform")
	}
	version := ""
	for _, line := range strings.Split(goMod, "\n") {
		p := strings.Fields(line)
		if len(p) == 2 && p[0] == "toolchain" {
			version = p[1]
		}
	}
	if version == "" {
		return "", fmt.Errorf("missing pinned host toolchain")
	}
	dir := filepath.Join(root, ".cache", "toolchains", version)
	exe := filepath.Join(dir, "go", "bin", "go.exe")
	if _, err := os.Stat(exe); err == nil {
		return exe, nil
	}
	client := &http.Client{Timeout: 3 * time.Minute}
	response, err := client.Get("https://go.dev/dl/?mode=json&include=all")
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "", fmt.Errorf("Go release metadata unavailable")
	}
	var releases []struct {
		Version string `json:"version"`
		Files   []struct{ Filename, OS, Arch, SHA256, Kind string }
	}
	if err = json.NewDecoder(io.LimitReader(response.Body, 5<<20)).Decode(&releases); err != nil {
		return "", err
	}
	filename, expected := "", ""
	for _, release := range releases {
		if release.Version == version {
			for _, file := range release.Files {
				if file.OS == "windows" && file.Arch == "amd64" && file.Kind == "archive" && strings.HasSuffix(file.Filename, ".zip") {
					filename, expected = file.Filename, file.SHA256
				}
			}
		}
	}
	if filename == "" || strings.ContainsAny(filename, "/\\") {
		return "", fmt.Errorf("pinned Go toolchain archive unavailable")
	}
	response, err = client.Get("https://go.dev/dl/" + filename)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return "", fmt.Errorf("Go archive download failed")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 512<<20+1))
	if err != nil || len(data) > 512<<20 {
		return "", fmt.Errorf("Go archive exceeds download limit")
	}
	sum := sha256.Sum256(data)
	if hex.EncodeToString(sum[:]) != strings.ToLower(expected) {
		return "", fmt.Errorf("Go toolchain SHA-256 mismatch")
	}
	if err = os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return "", err
	}
	stage, err := os.MkdirTemp(filepath.Dir(dir), ".toolchain-*")
	if err != nil {
		return "", err
	}
	archivePath := filepath.Join(stage, "toolchain.zip")
	if err = os.WriteFile(archivePath, data, 0600); err != nil {
		return "", err
	}
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	for _, f := range archive.File {
		if !strings.HasPrefix(f.Name, "go/") || strings.ContainsAny(f.Name, "\\:") || f.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("unsafe Go toolchain path")
		}
		target := filepath.Join(stage, filepath.FromSlash(f.Name))
		rel, err := filepath.Rel(stage, target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("unsafe Go toolchain path")
		}
		if f.FileInfo().IsDir() {
			if err = os.MkdirAll(target, 0755); err != nil {
				return "", err
			}
			continue
		}
		input, err := f.Open()
		if err != nil {
			return "", err
		}
		bytes, err := io.ReadAll(io.LimitReader(input, 64<<20+1))
		input.Close()
		if err != nil || len(bytes) > 64<<20 {
			return "", fmt.Errorf("toolchain entry too large")
		}
		if err = atomicFile(target, bytes); err != nil {
			return "", err
		}
	}
	archive.Close()
	_ = os.Remove(archivePath)
	if err = os.Rename(stage, dir); err != nil {
		return "", err
	}
	return exe, nil
}
