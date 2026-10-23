package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"hsmultiplayer/internal/session"
)

// Explicit file diagnostics run outside the simulation goroutine. Keep only
// the most recent two minutes; file IO can never hold a world-state lock.
func monitorServer(ctx context.Context, co *session.Coordinator, path string) func() {
	if path == "" {
		return func() {}
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var samples []session.Diagnostics
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s, err := co.Diagnostics()
				if err != nil {
					return
				}
				samples = append(samples, s)
				if len(samples) > 120 {
					samples = samples[len(samples)-120:]
				}
				data, err := json.MarshalIndent(struct {
					Note    string                `json:"timing_note"`
					Samples []session.Diagnostics `json:"samples"`
				}{"Inclusive host timings; do not add parents and children. P95 and GC pauses are histogram estimates. Network writes include socket wait.", samples}, "", "  ")
				if err == nil {
					err = os.WriteFile(path, data, 0600)
				}
				if err != nil {
					log.Printf("save server performance: %v", err)
				}
			}
		}
	}()
	return func() { cancel(); <-done }
}
