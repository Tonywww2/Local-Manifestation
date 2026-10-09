package server

import (
	"encoding/json"
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/mods"
	"hsmultiplayer/pkg/protocol"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"
)

type Distribution struct {
	Root string
	Lock mods.Lock
}

func (d Distribution) handle(conn net.Conn, h protocol.Hello) {
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	enc := json.NewEncoder(conn)
	if h.Version != protocol.Version {
		_ = enc.Encode(protocol.ErrorMessage(fmt.Errorf("incompatible protocol")))
		return
	}
	if h.Request == "mod_manifest" {
		lock := d.Lock
		if lock.Version == 0 {
			lock = mods.Lock{Version: 1}
		}
		data, _ := json.Marshal(lock)
		_ = enc.Encode(protocol.Message{Kind: "mod_manifest", Export: data})
		return
	}
	if h.Request != "mod_package" {
		return
	}
	allowed := false
	for _, p := range d.Lock.Packages {
		if p.SHA256 == h.SHA256 {
			allowed = true
		}
	}
	if !allowed {
		_ = enc.Encode(protocol.ErrorMessage(fmt.Errorf("package not in server combination")))
		return
	}
	path := filepath.Join(d.Root, h.SHA256, "package.hsmod")
	if _, _, err := content.ReadPackage(path, h.SHA256); err != nil {
		_ = enc.Encode(protocol.ErrorMessage(err))
		return
	}
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return
	}
	if err = enc.Encode(protocol.Message{Kind: "mod_package", Size: stat.Size()}); err != nil {
		return
	}
	_, _ = io.Copy(conn, f)
}
