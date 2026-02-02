//go:build !windows
// +build !windows

package memory

import (
	"os"
	"reflect"
	"runtime"
	"sync"

	"github.com/tencent/goom/internal/unexports2"
)

type stopTheWorldFn func(string)
type startTheWorldFn func()

var (
	stopOnce sync.Once
	stopFn   stopTheWorldFn
	startFn  startTheWorldFn
)

func initStopTheWorld() {
	stopPtr, err := unexports2.FindFuncByName("runtime.stopTheWorld")
	if err == nil && stopPtr != 0 {
		stopVal := unexports2.NewFuncWithCodePtr(reflect.TypeOf((stopTheWorldFn)(nil)), stopPtr)
		stopFn, _ = stopVal.Interface().(stopTheWorldFn)
	}
	startPtr, err := unexports2.FindFuncByName("runtime.startTheWorld")
	if err == nil && startPtr != 0 {
		startVal := unexports2.NewFuncWithCodePtr(reflect.TypeOf((startTheWorldFn)(nil)), startPtr)
		startFn, _ = startVal.Interface().(startTheWorldFn)
	}
}

func stopTheWorld(reason string) {
	stopOnce.Do(initStopTheWorld)
	if stopFn != nil {
		stopFn(reason)
	}
}

func startTheWorld() {
	stopOnce.Do(initStopTheWorld)
	if startFn != nil {
		startFn()
	}
}

func stwEnabled() bool {
	if runtime.GOARCH != "arm64" {
		return false
	}
	if os.Getenv("GOOM_STW_PATCH") == "0" {
		return false
	}
	return true
}
