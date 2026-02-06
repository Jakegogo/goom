// Package flags provides shared debug/dump flags for argdump implementations.
package flags

// DebugEnabled enables verbose diagnostics from callDump (stderr).
var DebugEnabled bool

// DumpEnabled controls whether argdump prints/encodes arguments/returns/panics.
var DumpEnabled = true

// DebugCallDumpPtr is the entry PC for callDump (diagnostics only).
var DebugCallDumpPtr uintptr

// DebugCallReflectPtr is the resolved entry PC for reflect.callReflect (diagnostics only).
var DebugCallReflectPtr uintptr

// DebugRuntimeReflectcallPtr is the entry PC for runtime.reflectcall (diagnostics only).
var DebugRuntimeReflectcallPtr uintptr

// DebugRuntimeSpillArgsPtr/DebugRuntimeUnspillArgsPtr are entry PCs for runtime.{spillArgs,unspillArgs}.
var DebugRuntimeSpillArgsPtr uintptr
var DebugRuntimeUnspillArgsPtr uintptr

// DebugMoveMakeFuncArgPtrsPtr is entry PC for reflect.moveMakeFuncArgPtrs (ABIInternal).
var DebugMoveMakeFuncArgPtrsPtr uintptr

// DebugLastProxyFuncValPtr is the last proxy func value pointer built by PatchFuncPtr.
var DebugLastProxyFuncValPtr uintptr

// DebugLastProxyCodePtr is the code pointer used for the makeFuncStub jump.
var DebugLastProxyCodePtr uintptr

// DebugLastMakeFuncStubPtr is the resolved code pointer used for reflect.makeFuncStub.
var DebugLastMakeFuncStubPtr uintptr

// DebugOrigFuncValPtr/DebugOrigCodePtr capture the original function value pointer and its code pointer.
var DebugOrigFuncValPtr uintptr
var DebugOrigCodePtr uintptr
