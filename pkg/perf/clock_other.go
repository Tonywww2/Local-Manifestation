//go:build !windows

package perf

import "time"

type Stamp struct{ at time.Time }

func Start() Stamp                      { return Stamp{time.Now()} }
func Elapsed(start Stamp) time.Duration { return time.Since(start.at) }
