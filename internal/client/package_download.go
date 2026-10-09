package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"hsmultiplayer/internal/content"
	"hsmultiplayer/pkg/mods"
	"hsmultiplayer/pkg/protocol"
	"io"
	"net"
	"os"
	"time"
)

func packageConnection(address string, hello protocol.Hello) (net.Conn, *bufio.Reader, protocol.Message, error) {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return nil, nil, protocol.Message{}, err
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	hello.Version = protocol.Version
	if err = json.NewEncoder(conn).Encode(hello); err != nil {
		conn.Close()
		return nil, nil, protocol.Message{}, err
	}
	reader := bufio.NewReaderSize(conn, 4096)
	var data []byte
	for len(data) <= protocol.MaxMessage {
		part, readErr := reader.ReadSlice('\n')
		data = append(data, part...)
		if readErr == bufio.ErrBufferFull {
			continue
		}
		err = readErr
		break
	}
	var message protocol.Message
	if err == nil && len(data) <= protocol.MaxMessage {
		err = json.Unmarshal(data, &message)
	} else if err == nil {
		err = fmt.Errorf("package header too large")
	}
	if err == nil && message.Kind == "error" {
		err = fmt.Errorf("server rejected package request: %s", message.Text)
	}
	if err != nil {
		conn.Close()
		return nil, nil, message, err
	}
	return conn, reader, message, nil
}
func DownloadPackages(address, root, lockPath string) error {
	conn, _, message, err := packageConnection(address, protocol.Hello{Request: "mod_manifest"})
	if err != nil {
		return err
	}
	conn.Close()
	if message.Kind != "mod_manifest" {
		return fmt.Errorf("invalid server manifest")
	}
	var lock mods.Lock
	if err = json.Unmarshal(message.Export, &lock); err != nil {
		return err
	}
	if lock.Version != 1 || len(lock.Packages) > 128 {
		return fmt.Errorf("invalid server package lock")
	}
	for _, p := range lock.Packages {
		conn, reader, message, err := packageConnection(address, protocol.Hello{Request: "mod_package", SHA256: p.SHA256})
		if err != nil {
			return err
		}
		if message.Kind != "mod_package" || message.Size < 1 || message.Size > 256<<20 {
			conn.Close()
			return fmt.Errorf("invalid package size")
		}
		f, err := os.CreateTemp("", "hsm-package-*.hsmod")
		if err != nil {
			conn.Close()
			return err
		}
		path := f.Name()
		n, copyErr := io.CopyN(f, reader, message.Size)
		closeErr := f.Close()
		conn.Close()
		if copyErr == nil && n != message.Size {
			copyErr = io.ErrUnexpectedEOF
		}
		if copyErr == nil {
			copyErr = closeErr
		}
		if copyErr == nil {
			record, installErr := content.InstallPackage(root, path, p.SHA256)
			copyErr = installErr
			if installErr == nil && (record.ID != p.ID || record.Version != p.Version) {
				copyErr = fmt.Errorf("server package metadata mismatch")
			}
		}
		_ = os.Remove(path)
		if copyErr != nil {
			return copyErr
		}
	}
	return content.SaveLock(lockPath, lock)
}
