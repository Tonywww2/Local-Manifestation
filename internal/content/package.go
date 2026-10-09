package content

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"
	"time"
)

const maxPackage = 256 << 20
const maxFile = 16 << 20

type PackageInfo struct {
	SHA256        string `json:"sha256"`
	Files         int    `json:"files"`
	ContainsGo    bool   `json:"contains_go"`
	HotReloadable bool   `json:"hot_reloadable"`
}

func Pack(files fs.FS) ([]byte, PackageInfo, error) {
	var names []string
	err := fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("symlinks not allowed: %s", path)
		}
		if !d.IsDir() {
			if !d.Type().IsRegular() {
				return fmt.Errorf("nonregular file %s", path)
			}
			names = append(names, path)
		}
		return nil
	})
	if err != nil {
		return nil, PackageInfo{}, err
	}
	sort.Strings(names)
	if len(names) == 0 || len(names) > 10000 {
		return nil, PackageInfo{}, fmt.Errorf("invalid file count")
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	total := 0
	for _, name := range names {
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return nil, PackageInfo{}, err
		}
		total += len(data)
		if len(data) > maxFile || total > maxPackage {
			return nil, PackageInfo{}, fmt.Errorf("package too large")
		}
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetModTime(time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC))
		h.SetMode(0644)
		entry, err := w.CreateHeader(h)
		if err != nil {
			return nil, PackageInfo{}, err
		}
		if _, err = entry.Write(data); err != nil {
			return nil, PackageInfo{}, err
		}
	}
	if err = w.Close(); err != nil {
		return nil, PackageInfo{}, err
	}
	sum := sha256.Sum256(b.Bytes())
	reader, info, err := Verify(b.Bytes(), hex.EncodeToString(sum[:]))
	_ = reader
	return b.Bytes(), info, err
}

// Verify requires an expected digest supplied by the selected content combination.
// SHA-256 establishes byte identity, not authorship or trust.
func Verify(data []byte, expected string) (*zip.Reader, PackageInfo, error) {
	if len(data) > maxPackage {
		return nil, PackageInfo{}, fmt.Errorf("package too large")
	}
	wanted, err := hex.DecodeString(expected)
	if err != nil || len(wanted) != sha256.Size {
		return nil, PackageInfo{}, fmt.Errorf("expected SHA-256 is required")
	}
	sum := sha256.Sum256(data)
	if !bytes.Equal(sum[:], wanted) {
		return nil, PackageInfo{}, fmt.Errorf("SHA-256 mismatch")
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, PackageInfo{}, err
	}
	info := PackageInfo{SHA256: hex.EncodeToString(sum[:]), HotReloadable: true}
	seen := map[string]bool{}
	folded := map[string]bool{}
	var total uint64
	for _, f := range r.File {
		name := strings.TrimSuffix(f.Name, "/")
		key := strings.ToLower(name)
		if folded[key] {
			return nil, PackageInfo{}, fmt.Errorf("case-colliding archive entry")
		}
		folded[key] = true
		for _, part := range strings.Split(name, "/") {
			base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
			if strings.HasSuffix(part, ".") || strings.HasSuffix(part, " ") || base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
				return nil, PackageInfo{}, fmt.Errorf("unsupported Windows archive path")
			}
		}
		if !fs.ValidPath(name) || strings.ContainsAny(name, "\\:") || f.Mode()&fs.ModeSymlink != 0 || seen[name] {
			return nil, PackageInfo{}, fmt.Errorf("unsafe or duplicate entry %s", f.Name)
		}
		seen[name] = true
		if f.FileInfo().IsDir() {
			continue
		}
		if !f.Mode().IsRegular() {
			return nil, PackageInfo{}, fmt.Errorf("nonregular entry")
		}
		if f.UncompressedSize64 > maxFile || total > maxPackage-f.UncompressedSize64 {
			return nil, PackageInfo{}, fmt.Errorf("expanded package too large")
		}
		total += f.UncompressedSize64
		info.Files++
		if info.Files > 10000 {
			return nil, PackageInfo{}, fmt.Errorf("too many files")
		}
		if strings.HasSuffix(strings.ToLower(name), ".go") {
			info.ContainsGo = true
			info.HotReloadable = false
		}
		in, err := f.Open()
		if err != nil {
			return nil, PackageInfo{}, err
		}
		n, readErr := io.Copy(io.Discard, io.LimitReader(in, maxFile+1))
		in.Close()
		if readErr != nil || n > maxFile {
			return nil, PackageInfo{}, fmt.Errorf("invalid archive entry %s", name)
		}
	}
	if _, ok := seen["manifest.json"]; !ok {
		return nil, PackageInfo{}, fmt.Errorf("missing manifest.json")
	}
	return r, info, nil
}
func ReadPackage(path, expected string) (*zip.Reader, PackageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, PackageInfo{}, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, maxPackage+1))
	if err != nil {
		return nil, PackageInfo{}, err
	}
	return Verify(b, expected)
}
