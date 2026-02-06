//go:build go1.13 && !go1.17
// +build go1.13,!go1.17

package pre117

import "github.com/tencent/goom/internal/argdump/internal/flags"

// Re-export flags from shared package for backward compatibility.
var (
	DebugEnabled                = &flags.DebugEnabled
	DumpEnabled                 = &flags.DumpEnabled
	DebugCallDumpPtr            = &flags.DebugCallDumpPtr
	DebugCallReflectPtr         = &flags.DebugCallReflectPtr
	DebugRuntimeReflectcallPtr  = &flags.DebugRuntimeReflectcallPtr
	DebugRuntimeSpillArgsPtr    = &flags.DebugRuntimeSpillArgsPtr
	DebugRuntimeUnspillArgsPtr  = &flags.DebugRuntimeUnspillArgsPtr
	DebugMoveMakeFuncArgPtrsPtr = &flags.DebugMoveMakeFuncArgPtrsPtr
	DebugLastProxyFuncValPtr    = &flags.DebugLastProxyFuncValPtr
	DebugLastProxyCodePtr       = &flags.DebugLastProxyCodePtr
	DebugLastMakeFuncStubPtr    = &flags.DebugLastMakeFuncStubPtr
	DebugOrigFuncValPtr         = &flags.DebugOrigFuncValPtr
	DebugOrigCodePtr            = &flags.DebugOrigCodePtr
)
