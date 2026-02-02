package patch

import (
	"encoding/binary"
	"testing"
)

func TestJmpToFunctionValue_ShortJump_LDRLiteralAndBR(t *testing.T) {
	from := uintptr(0x1000_0000)
	to := from + 4 + 0x100 // delta=0x100, imm19=0x40

	out := jmpToFunctionValue(from, to, 0)
	if len(out) != 8 {
		t.Fatalf("unexpected len=%d, want 8, out=%x", len(out), out)
	}

	ins := binary.LittleEndian.Uint32(out[0:4])
	imm19 := uint32((0x100) >> 2) // 0x40
	wantIns := uint32(0x58000000) | (imm19 << 5) | 10
	if ins != wantIns {
		t.Fatalf("unexpected LDR literal ins=0x%08x want=0x%08x out=%x", ins, wantIns, out)
	}

	br := binary.LittleEndian.Uint32(out[4:8])
	const wantBR = uint32(0xD61F0140) // BR X10
	if br != wantBR {
		t.Fatalf("unexpected BR ins=0x%08x want=0x%08x out=%x", br, wantBR, out)
	}
}

func TestJmpToFunctionValue_Fallback_LongForm(t *testing.T) {
	from := uintptr(0x1000_0000)
	to := from + 4 + (2 << 20) // 2MB away -> out of +/-1MB literal range

	out := jmpToFunctionValue(from, to, 0)
	if len(out) != 24 {
		t.Fatalf("unexpected len=%d, want 24, out=%x", len(out), out)
	}

	// ... MOVZ/MOVK x26 ... (16 bytes)
	// LDR x16, [x26] (4 bytes) then BR x16 (4 bytes)
	ldr := binary.LittleEndian.Uint32(out[16:20])
	if ldr != 0xF9400350 { // LDR X16, [X26]
		t.Fatalf("unexpected LDR [x26] ins=0x%08x want=0x%08x out=%x", ldr, uint32(0xF9400350), out)
	}
	br := binary.LittleEndian.Uint32(out[20:24])
	if br != 0xD61F0200 { // BR X16
		t.Fatalf("unexpected BR ins=0x%08x want=0x%08x out=%x", br, uint32(0xD61F0200), out)
	}
}
