// Package server provides a bounded TCP/JSON development transport.
package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"hsmultiplayer/internal/session"
	"hsmultiplayer/pkg/perf"
	"hsmultiplayer/pkg/protocol"
	"net"
	"sync"
	"time"
)

func Serve(ctx context.Context, l net.Listener, c *session.Coordinator, distribution ...Distribution) error {
	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	var mu sync.Mutex
	connections := map[net.Conn]bool{}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			l.Close()
			mu.Lock()
			for conn := range connections {
				conn.Close()
			}
			mu.Unlock()
		case <-done:
		}
	}()
	defer func() {
		cancel()
		l.Close()
		mu.Lock()
		for conn := range connections {
			conn.Close()
		}
		mu.Unlock()
		wg.Wait()
	}()
	for {
		conn, err := l.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		mu.Lock()
		connections[conn] = true
		mu.Unlock()
		if ctx.Err() != nil {
			conn.Close()
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { conn.Close(); mu.Lock(); delete(connections, conn); mu.Unlock() }()
			var packages Distribution
			if len(distribution) > 0 {
				packages = distribution[0]
			}
			handle(ctx, conn, c, packages)
		}()
	}
}
func handle(ctx context.Context, conn net.Conn, c *session.Coordinator, distribution Distribution) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), protocol.MaxMessage)
	if !scanner.Scan() {
		return
	}
	var h protocol.Hello
	if err := json.Unmarshal(scanner.Bytes(), &h); err != nil {
		return
	}
	if h.Request != "" {
		distribution.handle(conn, h)
		return
	}
	out := make(chan protocol.Message, 16)
	id, g, err := c.Connect(h, out)
	if err != nil {
		conn.SetWriteDeadline(time.Now().Add(time.Second))
		_ = json.NewEncoder(conn).Encode(protocol.ErrorMessage(err))
		return
	}
	defer c.Disconnect(id, g)
	conn.SetReadDeadline(time.Time{})
	done := make(chan struct{})
	writerStopped := make(chan struct{})
	defer func() { close(done); conn.Close(); <-writerStopped }()
	go func() {
		defer close(writerStopped)
		defer conn.Close()
		enc := json.NewEncoder(conn)
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case msg := <-out:
				conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
				writeStart := perf.Start()
				err := enc.Encode(msg)
				c.ObserveTransport("server.network.encode_write", perf.Elapsed(writeStart))
				if err != nil || msg.Kind == "replaced" {
					return
				}
			}
		}
	}()
	// Leave headroom above 60 Hz input for ping and normal UI commands.
	const requestsPerSecond = 80.0
	credits := requestsPerSecond * 2
	updated := time.Now()
	for scanner.Scan() {
		now := time.Now()
		credits += now.Sub(updated).Seconds() * requestsPerSecond
		if credits > requestsPerSecond*2 {
			credits = requestsPerSecond * 2
		}
		updated = now
		credits--
		if credits < 0 {
			return
		}
		decodeStart := perf.Start()
		var r protocol.Request
		if json.Unmarshal(scanner.Bytes(), &r) != nil {
			return
		}
		c.ObserveTransport("server.network.decode", perf.Elapsed(decodeStart))
		if r.Seq == 0 {
			return
		}
		c.Submit(id, g, r)
	}
}
func Listen(address string) (net.Listener, error) {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen: %w", err)
	}
	return l, nil
}
