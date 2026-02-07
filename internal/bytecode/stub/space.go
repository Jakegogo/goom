package stub

import (
	"fmt"

	"github.com/tencent/goom/internal/bytecode/memory"
)

const (
	// TypeHolder place holder 方式获取可执行内存
	TypeHolder = 1
	// TypeMMap mmap 方式获取可执行内存
	TypeMMap = 2
)

// Space 可执行空间
type Space struct {
	Addr  uintptr
	Space *[]byte
	typ   int
}

// Acquire enough executable space
// nolint
func Acquire(spaceLen int) (*Space, error) {
	if addr, space, err := acquireFromMMap(spaceLen); err == nil {
		return &Space{
			Addr:  addr,
			Space: space,
			typ:   TypeMMap,
		}, nil
	}
	if addr, space, err := acquireFromHolder(spaceLen); err == nil {
		return &Space{
			Addr:  addr,
			Space: space,
			typ:   TypeHolder,
		}, nil
	} else {
		return nil, err
	}
}

// AcquireFromHolder forces allocating executable space from the in-binary placeholder
// region instead of using mmap. This is useful on platforms where W^X/JIT restrictions
// make RWX mmap regions non-executable at runtime.
func AcquireFromHolder(spaceLen int) (*Space, error) {
	addr, space, err := acquireFromHolder(spaceLen)
	if err != nil {
		return nil, err
	}
	return &Space{
		Addr:  addr,
		Space: space,
		typ:   TypeHolder,
	}, nil
}

// Write 写入数据
func Write(s *Space, data []byte) error {
	switch s.typ {
	case TypeMMap:
		return writeToMMap(s.Addr, s.Space, data)
	case TypeHolder:
		return memory.WriteTo(s.Addr, data)
	default:
		return fmt.Errorf("stub write fail, illegal type: %d", s.typ)
	}
}
