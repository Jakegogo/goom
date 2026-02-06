//go:build go1.18 && !go1.24 && arm64
// +build go1.18,!go1.24,arm64

#include "textflag.h"

// makeFuncStubEntry is a tiny, stable entrypoint located in this module's text.
// The origin function is patched to jump here with x26 already set to our ctxt.
// We then tail-jump into reflect.makeFuncStub.
//
// This avoids requiring the origin patch to emit a long absolute jump sequence
// when reflect.makeFuncStub is out of arm64 B-imm26 range (±128MB).
TEXT ·makeFuncStubEntry(SB),NOSPLIT|NOFRAME,$0-0
	// Use a direct branch (B imm26) instead of an indirect JMP via register.
	// On darwin/arm64e, indirect branches to unauthenticated pointers are a common source of SIGBUS.
	// Keeping this as a direct branch materially improved stability during go1.17 PatchFunc rollout.
	B	reflect·makeFuncStub(SB)
