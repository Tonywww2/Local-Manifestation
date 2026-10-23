package perf

import (
	"syscall"
	"time"
	"unsafe"
)

type Stamp struct{ counter int64 }

var counterProc = syscall.NewLazyDLL("kernel32.dll").NewProc("QueryPerformanceCounter")
var counterFrequency = func() float64 {
	var value int64
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("QueryPerformanceFrequency")
	if ok, _, _ := proc.Call(uintptr(unsafe.Pointer(&value))); ok == 0 || value <= 0 {
		panic("Windows high-resolution performance counter unavailable")
	}
	return float64(value)
}()

// Start uses Windows' high-resolution monotonic counter. Short server phases
// must not disappear below the wall-clock sampling resolution.
func Start() Stamp {
	var value int64
	counterProc.Call(uintptr(unsafe.Pointer(&value)))
	return Stamp{value}
}
func Elapsed(start Stamp) time.Duration {
	return time.Duration(float64(Start().counter-start.counter) * 1e9 / counterFrequency)
}
