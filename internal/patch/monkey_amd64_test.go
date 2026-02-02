package patch

import (
	"encoding/binary"
	"testing"
)

func TestJmpToFunctionValue_ShortJump_RIPRelativeIndirect(t *testing.T) {
	from := uintptr(0x100000000)
	to := from + 0x200 // within int32 RIP-relative range

	out := jmpToFunctionValue(from, to, 0)
	if len(out) != 7 {
		t.Fatalf("unexpected len=%d, want 7, out=%x", len(out), out)
	}
	if out[0] != nopOpcode {
		t.Fatalf("unexpected marker=%#x, want %#x", out[0], nopOpcode)
	}
	if out[1] != 0xFF || out[2] != 0x25 {
		t.Fatalf("unexpected opcode prefix=%x %x, want ff 25, out=%x", out[1], out[2], out)
	}

	disp := int32(binary.LittleEndian.Uint32(out[3:7]))
	base := int64(from + 7) // NOP(1) + JMP(6)
	gotTo := uintptr(base + int64(disp))
	if gotTo != to {
		t.Fatalf("unexpected disp: gotTo=0x%x wantTo=0x%x disp=%d out=%x", gotTo, to, disp, out)
	}
}

func TestJmpToFunctionValue_Fallback_LongForm(t *testing.T) {
	from := uintptr(0x1000)
	to := uintptr(0x7fff_ffff_ffff_ffff) // force out-of-range for disp32

	out := jmpToFunctionValue(from, to, 0)
	if len(out) != 13 {
		t.Fatalf("unexpected len=%d, want 13, out=%x", len(out), out)
	}
	if out[0] != nopOpcode {
		t.Fatalf("unexpected marker=%#x, want %#x", out[0], nopOpcode)
	}
	// NOP; MOVABS RDX, imm64; JMP QWORD PTR [RDX]
	if out[1] != 0x48 || out[2] != 0xBA {
		t.Fatalf("unexpected movabs prefix=%x %x, want 48 ba, out=%x", out[1], out[2], out)
	}
	if out[11] != 0xFF || out[12] != 0x22 {
		t.Fatalf("unexpected jmp [rdx] suffix=%x %x, want ff 22, out=%x", out[11], out[12], out)
	}
}
