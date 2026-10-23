package perf

import (
	"os"
	"runtime/pprof"
)

// StartCPUProfile is an explicit developer-only side-effect boundary.
func StartCPUProfile(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err = pprof.StartCPUProfile(f); err != nil {
		f.Close()
		return nil, err
	}
	return func() { pprof.StopCPUProfile(); f.Close() }, nil
}
