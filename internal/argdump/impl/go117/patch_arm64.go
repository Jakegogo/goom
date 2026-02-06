//go:build go1.17 && !go1.18 && arm64
// +build go1.17,!go1.18,arm64

package go117

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
	"unsafe"

	"github.com/tencent/goom/internal/argdump/abi"
	"github.com/tencent/goom/internal/argdump/internal/flags"
	"github.com/tencent/goom/internal/argdump/internal/util"
	"github.com/tencent/goom/internal/bytecode"
	"github.com/tencent/goom/internal/patch"
	"github.com/tencent/goom/internal/unexports2"
)

// makeFuncStubEntry is an arm64 assembly stub that tail-jumps into reflect.makeFuncStub.
// It exists to keep the origin entry patch small (20 bytes) even when reflect.makeFuncStub
// is >128MB away (arm64 B imm26 range), which would otherwise force a longer patch (36 bytes)
// and risk overwriting past small function bodies on go1.17.
//
// Deployment note (go1.17/darwin/arm64):
//   - We patch the *target function entry* to branch to this local stub (usually placed near other argdump code),
//     so the branch is always in-range and the patch stays short.
//   - This stub then branches into the real stdlib reflect.makeFuncStub.
//   - The ctxt register (x26) is set at the patched origin entry by patch.PtrCodeWithCtx*, so makeFuncStub sees
//     our makeFuncCtxt/dumpFuncImpl as its closure context.
func makeFuncStubEntry()


// guardKeepAlive retains patch-specific contexts (like *dumpFuncImpl) so the GC
// doesn't reclaim them while patched code still references their addresses.
//
// On arm64, the patched entry stub sets x26=ctxtPtr via an immediate constant, which
// is NOT visible to the GC as a heap reference. Without an external heap reference,
// GC may free the ctxt object and the next call will crash.
var guardKeepAlive sync.Map // map[*patch.Guard]any

// PatchFunc patches the provided function so that every call will:
// - print all arguments (via abijson) using the arg frame addresses
// - call the original function (by temporarily unpatching) and preserve returns
//
// go1.17/arm64 note:
// reflect.makeFuncStub passes regs=nil (stack-only path), so we use a stack-only
// ABI description and call the original via reflect.Call (avoids runtime.reflectcall
// frame-type complexity on this Go version).
//
// Key stability changes for go1.17/darwin/arm64:
//   - **Do not unpatch/restore inside callDump**: on Apple Silicon, writing .text may involve temporarily removing
//     EXEC from an entire page. If the patched target shares a page with currently executing code, this can crash.
//   - Instead, we call the original via **fix-origin trampoline** (Guard.FixOriginFunc) and keep the patch applied.
func PatchFunc(fn interface{}) (*patch.Guard, error) {
	if fn == nil {
		return nil, errors.New("argdump: PatchFunc nil")
	}
	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return nil, fmt.Errorf("argdump: PatchFunc expects func, got %s", t.Kind())
	}
	v := reflect.ValueOf(fn)
	originPtr := v.Pointer()
	origFuncVal := bytecode.GetPtr(v)
	flags.DebugOrigFuncValPtr = uintptr(origFuncVal)
	flags.DebugOrigCodePtr = *(*uintptr)(origFuncVal)
	return PatchFuncPtr(originPtr, origFuncVal, t)
}

// PatchFuncPtr is PatchFunc but takes an explicit function entry address and function type.
func PatchFuncPtr(originPtr uintptr, origFuncVal unsafe.Pointer, typ reflect.Type) (*patch.Guard, error) {
	if originPtr == 0 {
		return nil, errors.New("argdump: PatchFuncPtr originPtr=0")
	}
	if typ == nil || typ.Kind() != reflect.Func {
		return nil, errors.New("argdump: PatchFuncPtr typ must be a func type")
	}

	ensureCallReflectPatched()

	// IMPORTANT:
	// - ctxt.fn must point at the real reflect.makeFuncStub so the runtime's makeFuncStub stack map
	//   special-casing works correctly.
	// - the origin patch jump target can be our local stub entry (close to origin), which then
	//   tail-jumps into reflect.makeFuncStub.
	makeFuncStubPtr, err := unexports2.FindFuncByName("reflect.makeFuncStub")
	if err != nil {
		return nil, err
	}
	makeFuncStubEntryPtr := reflect.ValueOf(makeFuncStubEntry).Pointer()
	if flags.DebugEnabled {
		from := originPtr + 16
		delta := int64(makeFuncStubEntryPtr) - int64(from)
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] PatchFuncPtr origin=0x%x stubEntry=0x%x delta=%d (bytes)\n",
			originPtr, makeFuncStubEntryPtr, delta,
		)
	}

	abid := newAbiDescFromFuncType(typ)
	impl := &DumpFuncImpl{
		makeFuncCtxt: makeFuncCtxt{
			fn:     makeFuncStubPtr,
			stack:  abid.stackPtrs,
			argLen: abid.stackCallArgsSize,
			// go1.17 makeFuncStub passes regs=nil.
			regPtrs: intArgRegBitmap{},
		},
		magic:  uintptr(unsafe.Pointer(magicSentinel)),
		fnType: typ,
		abid:   abid,
	}

	// Preserve the original function value for reflect.Call.
	// For go1.17/darwin/arm64, calling original by temporarily unpatching can crash
	// because mprotect removes EXEC on the whole page while writing bytes.
	// So we call the original via a fixed-origin trampoline function (same signature).
	//
	// Note (closures):
	// A closure's func value carries a non-zero ctxt pointer (captured env). If we drop it,
	// the original function will observe a nil/garbage closure context and can return wrong
	// results or crash. So we propagate the original ctxt into the trampoline funcval below.

	ctxtPtr := uintptr(unsafe.Pointer(impl))
	flags.DebugLastProxyFuncValPtr = ctxtPtr
	flags.DebugLastProxyCodePtr = makeFuncStubEntryPtr
	flags.DebugLastMakeFuncStubPtr = makeFuncStubPtr

	guard, err := patch.PtrCodeWithCtxTrampoline(originPtr, ctxtPtr, makeFuncStubEntryPtr)
	if err != nil {
		return nil, err
	}
	impl.HookGuard = guard
	if flags.DebugEnabled {
		_, _ = fmt.Fprintf(os.Stderr, "[argdump] PatchFuncPtr guard fixOrigin=0x%x\n", guard.FixOriginFunc())
	}
	if guard.FixOriginFunc() == 0 {
		return nil, errors.New("argdump: PatchFunc trampoline not available (FixOriginFunc=0)")
	}

	// Build a minimal funcval pointing at the trampoline code pointer.
	type funcval struct {
		fn   uintptr
		ctxt uintptr
	}
	origCtxt := uintptr(0)
	if origFuncVal != nil {
		origCtxt = *(*uintptr)(unsafe.Add(origFuncVal, abi.PtrSize))
	}
	fv := &funcval{fn: guard.FixOriginFunc(), ctxt: origCtxt}
	impl.OrigFuncValBox = fv
	impl.OrigFn = util.PackEface(util.RtypePtr(typ), unsafe.Pointer(fv))

	guardKeepAlive.Store(guard, impl)
	runtime.SetFinalizer(guard, func(g *patch.Guard) {
		guardKeepAlive.Delete(g)
	})
	return guard, nil
}
