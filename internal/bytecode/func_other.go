//go:build !amd64 && !arm64
// +build !amd64,!arm64

package bytecode

import (
	"encoding/hex"

	"github.com/tencent/goom/internal/logger"
)

// PrintInstf prints instruction bytes for architectures where we don't have a decoder.
func PrintInstf(title string, from uintptr, copyOrigin []byte, level int) {
	if logger.LogLevel < level {
		return
	}
	_ = from
	logger.Consolef(level, "%s%s", title, hex.EncodeToString(copyOrigin))
}

// GetFuncSize is a best-effort fallback for non-amd64/arm64 architectures.
// It returns a conservative default to keep patch safety checks working.
func GetFuncSize(_ int, _ uintptr, _ bool) (length int, err error) {
	return 1024, nil
}

// GetInnerFunc is a best-effort fallback for non-amd64/arm64 architectures.
// For these ports we don't attempt wrapper decoding; return 0 to indicate "no inner".
func GetInnerFunc(_ int, _ uintptr) (uintptr, error) {
	return 0, nil
}


