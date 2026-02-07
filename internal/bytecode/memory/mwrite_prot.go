//go:build !windows && (amd64 || arm64)
// +build !windows
// +build amd64 arm64

package memory

import (
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"unsafe"
)

func writeTo(addr uintptr, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	// WHY: On darwin/arm64, mprotect+write can SIGBUS if we modify a text page
	// that is actively executing (target page == caller page). The fix is to
	// stop-the-world during patching so no goroutine executes from that page.
	// This is enabled by default for arm64 (GOOM_STW_PATCH=0 disables).
	if stwEnabled() {
		stopTheWorld("goom mwrite")
		defer startTheWorld()
	}
	trace := os.Getenv("GOOM_TRACE_MPROTECT") == "1"
	traceStacks := os.Getenv("GOOM_TRACE_MPROTECT_STACK") == "1"
	pageSize := uintptr(syscall.Getpagesize())
	pageSizeInt := int(pageSize)
	begin := addr
	end := addr + uintptr(len(data))
	offset := 0
	for begin < end {
		beginPage := PageStart(begin)
		endPage := PageStart(end - 1)
		if beginPage != endPage {
			nextPage := beginPage + pageSize
			chunkLen := int(nextPage - begin)
			chunk := data[offset : offset+chunkLen]
			if trace {
				traceMprotectHeader(beginPage, pageSize, begin, chunkLen)
				traceCallerPages(beginPage, pageSize, begin, chunkLen)
			}
			if traceStacks {
				traceAllStacks(beginPage, begin, chunkLen)
			}
			res := write(begin, ptrOfBytes(chunk), chunkLen, beginPage, pageSizeInt, syscall.PROT_READ|syscall.PROT_EXEC)
			if res != 0 {
				return fmt.Errorf("write failed, code %v", res)
			}
			begin += uintptr(chunkLen)
			offset += chunkLen
			continue
		}
		chunk := data[offset:]
		if trace {
			traceMprotectHeader(beginPage, pageSize, begin, len(chunk))
			traceCallerPages(beginPage, pageSize, begin, len(chunk))
		}
		if traceStacks {
			traceAllStacks(beginPage, begin, len(chunk))
		}
		res := write(begin, ptrOfBytes(chunk), len(chunk), beginPage, pageSizeInt, syscall.PROT_READ|syscall.PROT_EXEC)
		if res != 0 {
			return fmt.Errorf("write failed, code %v", res)
		}
		break
	}
	return nil
}

func ptrOfBytes(data []byte) uintptr {
	if len(data) == 0 {
		return 0
	}
	return uintptr(unsafe.Pointer(&data[0]))
}

func traceMprotectHeader(page, pageSize, addr uintptr, length int) {
	targetFn := runtime.FuncForPC(addr)
	targetName := "<unknown>"
	if targetFn != nil {
		targetName = targetFn.Name()
	}
	pc, _, _, ok := runtime.Caller(2)
	callerName := "<unknown>"
	if ok {
		if fn := runtime.FuncForPC(pc); fn != nil {
			callerName = fn.Name()
		}
	}
	_, _ = fmt.Fprintf(os.Stderr, "[mprotect] target=0x%x addr=0x%x len=0x%x targetFunc=%s callerPC=0x%x callerFunc=%s\n",
		page, addr, length, targetName, pc, callerName)
	_ = pageSize
}

func traceAllStacks(page, addr uintptr, length int) {
	buf := make([]byte, 1<<20)
	n := runtime.Stack(buf, true)
	if n == 0 {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, "[mprotect] stacks target=0x%x addr=0x%x len=0x%x\n", page, addr, length)
	_, _ = os.Stderr.Write(buf[:n])
}

func traceCallerPages(page uintptr, pageSize uintptr, addr uintptr, length int) {
	pcs := make([]uintptr, 32)
	n := runtime.Callers(2, pcs)
	targetFn := runtime.FuncForPC(addr)
	targetName := "<unknown>"
	if targetFn != nil {
		targetName = targetFn.Name()
	}
	targetIsRuntime := strings.HasPrefix(targetName, "runtime.")
	hookSiteName := "<unknown>"
	var hookSitePC uintptr
	for i := 0; i < n; i++ {
		pc := pcs[i]
		if pc == 0 {
			continue
		}
		fn := runtime.FuncForPC(pc)
		if fn == nil {
			continue
		}
		name := fn.Name()
		if strings.HasPrefix(name, "runtime.") ||
			strings.HasPrefix(name, "github.com/tencent/goom/internal/bytecode/memory.") {
			continue
		}
		hookSiteName = name
		hookSitePC = pc
		break
	}
	for i := 0; i < n; i++ {
		pc := pcs[i]
		if pc == 0 {
			continue
		}
		callerPage := pc & ^(pageSize - 1)
		if callerPage != page {
			continue
		}
		callerFn := runtime.FuncForPC(pc)
		callerName := "<unknown>"
		if callerFn != nil {
			callerName = callerFn.Name()
		}
		callerIsRuntime := strings.HasPrefix(callerName, "runtime.")
		runtimeHit := targetIsRuntime || callerIsRuntime
		if !runtimeHit {
			continue
		}
		_, _ = fmt.Fprintf(os.Stderr, "[mprotect] target=0x%x addr=0x%x len=0x%x targetFunc=%s callerPC=0x%x callerPage=0x%x callerFunc=%s pcOffset=0x%x addrOffset=0x%x runtimeHit=%t hookSitePC=0x%x hookSiteFunc=%s\n",
			page, addr, length, targetName, pc, callerPage, callerName, pc-page, addr-page, runtimeHit, hookSitePC, hookSiteName)
	}
}

// write is implemented in platform-specific assembly.
func write(target, data uintptr, len int, page uintptr, pageSize, oriProt int) int
