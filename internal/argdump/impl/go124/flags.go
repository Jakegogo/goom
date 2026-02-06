//go:build go1.24
// +build go1.24

package go124

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
