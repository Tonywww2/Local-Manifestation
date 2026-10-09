package content

import (
	"bytes"
	"io"
	"io/fs"
	"path"
	"sort"
	"strings"
	"time"
)

// memoryFS is a loading-boundary value. Its private byte slices are frozen
// before the catalog is parsed, compiled or hashed.
type memoryFS map[string][]byte
type memoryFile struct {
	*bytes.Reader
	info    memoryInfo
	entries []fs.DirEntry
	offset  int
}
type memoryInfo struct {
	name string
	size int64
	dir  bool
}

func (v memoryInfo) Name() string { return v.name }
func (v memoryInfo) Size() int64  { return v.size }
func (v memoryInfo) Mode() fs.FileMode {
	if v.dir {
		return fs.ModeDir | 0555
	}
	return 0444
}
func (v memoryInfo) ModTime() time.Time          { return time.Time{} }
func (v memoryInfo) IsDir() bool                 { return v.dir }
func (v memoryInfo) Sys() any                    { return nil }
func (v memoryInfo) Type() fs.FileMode           { return v.Mode().Type() }
func (v memoryInfo) Info() (fs.FileInfo, error)  { return v, nil }
func (f *memoryFile) Close() error               { return nil }
func (f *memoryFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *memoryFile) ReadDir(n int) ([]fs.DirEntry, error) {
	if !f.info.dir {
		return nil, &fs.PathError{Op: "readdir", Path: f.info.name, Err: fs.ErrInvalid}
	}
	if n > 0 && f.offset == len(f.entries) {
		return nil, io.EOF
	}
	end := len(f.entries)
	if n > 0 {
		end = min(end, f.offset+n)
	}
	out := append([]fs.DirEntry(nil), f.entries[f.offset:end]...)
	f.offset = end
	return out, nil
}
func (m memoryFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	if data, ok := m[name]; ok {
		return &memoryFile{Reader: bytes.NewReader(data), info: memoryInfo{name: path.Base(name), size: int64(len(data))}}, nil
	}
	prefix := ""
	if name != "." {
		prefix = name + "/"
	}
	children := map[string]memoryInfo{}
	for file, data := range m {
		if !strings.HasPrefix(file, prefix) {
			continue
		}
		rel := strings.TrimPrefix(file, prefix)
		part, _, dir := strings.Cut(rel, "/")
		children[part] = memoryInfo{name: part, size: int64(len(data)), dir: dir}
	}
	if len(children) == 0 && name != "." {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	var names []string
	for child := range children {
		names = append(names, child)
	}
	sort.Strings(names)
	var entries []fs.DirEntry
	for _, child := range names {
		entries = append(entries, children[child])
	}
	return &memoryFile{Reader: bytes.NewReader(nil), info: memoryInfo{name: path.Base(name), dir: true}, entries: entries}, nil
}

func snapshotFiles(source fs.FS) (memoryFS, error) {
	out := memoryFS{}
	total := int64(0)
	err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			return &fs.PathError{Op: "snapshot", Path: name, Err: fs.ErrInvalid}
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() || len(out) >= 10000 {
			return &fs.PathError{Op: "snapshot", Path: name, Err: fs.ErrInvalid}
		}
		f, err := source.Open(name)
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(f, maxFile+1))
		f.Close()
		if err != nil {
			return err
		}
		total += int64(len(data))
		if len(data) > maxFile || total > maxPackage {
			return &fs.PathError{Op: "snapshot", Path: name, Err: fs.ErrInvalid}
		}
		out[name] = data
		return nil
	})
	return out, err
}
