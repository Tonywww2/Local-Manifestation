// Package testcontent creates validated test variants through the real loader.
package testcontent

import (
	"encoding/json"
	"io/fs"
	"testing"
	"testing/fstest"

	base "hsmultiplayer/content"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/model"
)

func Files(t testing.TB) fstest.MapFS {
	t.Helper()
	files := fstest.MapFS{}
	if err := fs.WalkDir(base.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		b, err := fs.ReadFile(base.Files, path)
		files[path] = &fstest.MapFile{Data: b}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
func Load(t testing.TB, edit func(*content.Definitions, *model.AdventureDef), script string) *content.Catalog {
	t.Helper()
	files := Files(t)
	var d content.Definitions
	var a model.AdventureDef
	if err := content.Decode(files["definitions.json"].Data, &d); err != nil {
		t.Fatal(err)
	}
	if err := content.Decode(files["adventure.json"].Data, &a); err != nil {
		t.Fatal(err)
	}
	if edit != nil {
		edit(&d, &a)
	}
	write := func(path string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = &fstest.MapFile{Data: b}
	}
	write("definitions.json", d)
	write("adventure.json", a)
	if script != "" {
		files["behavior.js"].Data = append(files["behavior.js"].Data, []byte("\n"+script)...)
	}
	c, err := content.Load(files)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
