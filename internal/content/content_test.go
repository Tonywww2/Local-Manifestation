package content

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	base "hsmultiplayer/content"
	"os"
	"testing"
	"testing/fstest"
)

func TestCommentsAndStrictFields(t *testing.T) {
	var dst struct {
		Number int `json:"number"`
	}
	if err := Decode([]byte(`{"number":2,"_comment":"ok"}`), &dst); err != nil || dst.Number != 2 {
		t.Fatal(dst, err)
	}
	for _, data := range []string{`{"numbre":2}`, `{"number":2} {}`} {
		if Decode([]byte(data), &dst) == nil {
			t.Fatal("accepted", data)
		}
	}
}
func TestCatalogIdentity(t *testing.T) {
	a, err := Load(base.Files)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Load(base.Files)
	if a.Digest != b.Digest {
		t.Fatal("unstable digest")
	}
	if len(a.Programs()) != 1 {
		t.Fatal("missing script")
	}
	external, err := Load(os.DirFS("../../content/base"))
	if err != nil || external.Digest != a.Digest {
		t.Fatal("embedded and external original content diverged", err)
	}
	data, info, err := Pack(os.DirFS("../../content/base"))
	if err != nil || info.ContainsGo || !info.HotReloadable {
		t.Fatal("original content includes an engine wrapper", info, err)
	}
	archive, _, err := Verify(data, info.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	packed, err := Load(archive)
	if err != nil || packed.Digest != a.Digest {
		t.Fatal("packed and embedded content diverged", err)
	}
}
func TestPackageDigestAndPaths(t *testing.T) {
	files := fstest.MapFS{"manifest.json": &fstest.MapFile{Data: []byte(`{}`)}, "main.js": &fstest.MapFile{Data: []byte("// content")}}
	b, info, err := Pack(files)
	if err != nil {
		t.Fatal(err)
	}
	b2, info2, err := Pack(files)
	if err != nil || !bytes.Equal(b, b2) || info.SHA256 != info2.SHA256 {
		t.Fatal("nondeterministic packing")
	}
	if !info.HotReloadable {
		t.Fatal(info)
	}
	bad := append([]byte(nil), b...)
	bad[len(bad)/2] ^= 1
	if _, _, err = Verify(bad, info.SHA256); err == nil {
		t.Fatal("accepted tampering")
	}
	if _, _, err = Verify(b, ""); err == nil {
		t.Fatal("accepted no digest")
	}
	files["native.go"] = &fstest.MapFile{Data: []byte("package mod")}
	_, info, err = Pack(files)
	if err != nil || info.HotReloadable || !info.ContainsGo {
		t.Fatal(info, err)
	}
	var unsafe bytes.Buffer
	z := zip.NewWriter(&unsafe)
	w, _ := z.Create("../manifest.json")
	w.Write([]byte(`{}`))
	z.Close()
	sum := sha256.Sum256(unsafe.Bytes())
	if _, _, err = Verify(unsafe.Bytes(), hex.EncodeToString(sum[:])); err == nil {
		t.Fatal("accepted traversal")
	}
}
