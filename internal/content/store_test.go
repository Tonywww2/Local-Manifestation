package content_test

import (
	"encoding/json"
	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/internal/testcontent"
	"io/fs"
	"sync"
	"testing"
	"testing/fstest"
)

type delayedFS struct {
	fs.FS
	entered, proceed chan struct{}
	once             sync.Once
}

func (d *delayedFS) Open(name string) (fs.File, error) {
	d.once.Do(func() { close(d.entered); <-d.proceed })
	return d.FS.Open(name)
}
func TestAdventureStartingDuringLoadPreventsPublication(t *testing.T) {
	c := testcontent.Load(t, nil, "")
	store := content.NewStore(c)
	files := &delayedFS{FS: base.Files, entered: make(chan struct{}), proceed: make(chan struct{})}
	result := make(chan error, 1)
	go func() { result <- store.Reload(files) }()
	<-files.entered
	pinned, release := store.Acquire()
	defer release()
	close(files.proceed)
	if err := <-result; err == nil || store.Current() != c || pinned != c {
		t.Fatal("published during adventure", err)
	}
}

type replacingFS struct {
	fstest.MapFS
	replacement []byte
	reads       int
}

func (f *replacingFS) ReadFile(name string) ([]byte, error) {
	data, err := fs.ReadFile(f.MapFS, name)
	if name == "definitions.json" {
		f.reads++
		f.MapFS[name].Data = f.replacement
	}
	return data, err
}
func TestDigestUsesTheValidatedDefinitionBytes(t *testing.T) {
	original, err := content.Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	files := testcontent.Files(t)
	var changed content.Definitions
	if err := content.Decode(files["definitions.json"].Data, &changed); err != nil {
		t.Fatal(err)
	}
	changed.Enemies[0].HP += 100
	replacement, err := json.Marshal(changed)
	if err != nil {
		t.Fatal(err)
	}
	source := &replacingFS{MapFS: files, replacement: replacement}
	loaded, err := content.Load(source)
	if err != nil {
		t.Fatal(err)
	}
	enemy := changed.Enemies[0].ID
	if source.reads != 1 || loaded.Digest != original.Digest || loaded.Enemy(enemy).HP != original.Enemy(enemy).HP {
		t.Fatal("hashed different bytes from validated content", source.reads, loaded.Digest, original.Digest)
	}
	later, err := content.Load(files)
	if err != nil || later.Digest == loaded.Digest {
		t.Fatal("replacement was not observed on next load", err)
	}
}
