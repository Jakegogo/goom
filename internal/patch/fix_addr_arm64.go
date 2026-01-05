package patch

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/tencent/goom/internal/arch/arm64asm"
	"github.com/tencent/goom/internal/logger"
)

const arm64InsnLen = 4

// fixRelativeAddr 修复函数字节码中的相对地址(如果有的话) - arm64
// from 函数起始地址
// copyOrigin 函数字节码
// trampoline 需要移动到的目标地址
// funcSize 函数字节码整体长度
// leastSize 要替换的字节长度的最小限制
func fixRelativeAddr(from uintptr, copyOrigin []byte, trampoline uintptr, funcSize int, leastSize int) (
	fixedData []byte, fixedDataSize int, err error) {
	// try to replace and get the len(endPos) to replace
	_, fixedDataSize, err = fixBlockArm64(from, copyOrigin, trampoline, leastSize, funcSize)
	if err != nil {
		return
	}

	// check if exists jump into [0:fixedDataSize], return not support
	if err = checkJumpBetweenArm64(from, fixedDataSize, copyOrigin, funcSize); err != nil {
		return
	}

	logger.Debugf("fix size: %d", fixedDataSize)

	// real replace
	fixedData, _, err = fixBlockArm64(from, copyOrigin, trampoline, fixedDataSize, fixedDataSize)
	return
}

func fixBlockArm64(from uintptr, block []byte, trampoline uintptr,
	leastSize int, blockSize int) (fixedData []byte, fixedDataSize int, err error) {
	fixedBlock := make([]byte, 0, leastSize+32)
	logger.Debug("target fix ins >>>>>")

	// arm64 is fixed-length, but keep it robust in case of odd tail padding.
	limit := len(block)
	if blockSize > 0 && blockSize < limit {
		limit = blockSize
	}

	for pos := 0; pos+arm64InsnLen <= limit; pos += arm64InsnLen {
		insWord := binary.LittleEndian.Uint32(block[pos : pos+arm64InsnLen])

		newWord, rewritten, e := rewritePCRelArm64(insWord, from+uintptr(pos), trampoline+uintptr(pos))
		if e != nil {
			return nil, 0, e
		}

		out := make([]byte, 4)
		binary.LittleEndian.PutUint32(out, newWord)
		fixedBlock = append(fixedBlock, out...)

		// debug print
		if logger.LogLevel <= logger.DebugLevel {
			code := block[pos : pos+arm64InsnLen]
			ins, derr := arm64asm.Decode(code)
			if derr == nil && ins.Op != 0 {
				if rewritten {
					logger.Debugf("[4]>[4] 0x%x:\t%s\t\t%-30s\t\t%s\t\t(rewritten: %s)",
						(uint64)(from)+uint64(pos), ins.Op, ins.String(), hex.EncodeToString(code),
						hex.EncodeToString(out))
				} else {
					logger.Debugf("[4]>[4] 0x%x:\t%s\t\t%-30s\t\t%s",
						(uint64)(from)+uint64(pos), ins.Op, ins.String(), hex.EncodeToString(code))
				}
			}
		}

		// stop after copying enough instructions, but avoid cutting right before a RET-only tail.
		if leastSize > 0 && (pos+arm64InsnLen) >= leastSize {
			nextPos := pos + arm64InsnLen
			if nextPos+arm64InsnLen <= limit {
				nextIns, derr := arm64asm.Decode(block[nextPos : nextPos+arm64InsnLen])
				if derr == nil && nextIns.Op != 0 && nextIns.Op.String() != "RET" {
					return fixedBlock, nextPos, nil
				}
			}
		}
	}

	return fixedBlock, len(fixedBlock), nil
}

// checkJumpBetweenArm64 checks if exists a branch instruction in originData of function
// that jumps into address range (from, from+to).
func checkJumpBetweenArm64(from uintptr, to int, originData []byte, funcSize int) error {
	limit := len(originData)
	if funcSize > 0 && funcSize < limit {
		limit = funcSize
	}
	for pos := 0; pos+arm64InsnLen <= limit; pos += arm64InsnLen {
		insWord := binary.LittleEndian.Uint32(originData[pos : pos+arm64InsnLen])
		if !isControlFlowPCRelArm64(insWord) {
			continue
		}
		rel, ok := decodeBranchOffsetArm64(insWord)
		if !ok {
			continue
		}
		targetPos := pos + int(rel)
		if targetPos > 0 && targetPos < to {
			code := originData[pos : pos+arm64InsnLen]
			ins, _ := arm64asm.Decode(code)
			logger.Errorf("[4] 0x%x:\t%s\t\t%-30s\t\t%s\t\tabs:0x%x",
				from+uintptr(pos), ins.Op, ins.String(), hex.EncodeToString(code),
				from+uintptr(targetPos))
			return fmt.Errorf("not support: branch jumps into the first %d bytes, jump address is: 0x%x",
				to, from+uintptr(targetPos))
		}
	}
	return nil
}

func rewritePCRelArm64(word uint32, originPC uintptr, trampPC uintptr) (newWord uint32, rewritten bool, err error) {
	// B / BL
	if isBArm64(word) || isBLArm64(word) {
		target := int64(originPC) + int64(signExtend(word&0x03FFFFFF, 26)<<2)
		imm := (target - int64(trampPC)) >> 2
		if (target-int64(trampPC))%4 != 0 {
			return 0, false, fmt.Errorf("arm64 branch target not aligned: originPC=0x%x trampPC=0x%x", originPC, trampPC)
		}
		if imm < -(1<<25) || imm >= (1<<25) {
			return 0, false, fmt.Errorf("arm64 B/BL out of range after relocation: originPC=0x%x trampPC=0x%x target=0x%x",
				originPC, trampPC, uintptr(target))
		}
		return (word & 0xFC000000) | (uint32(imm) & 0x03FFFFFF), true, nil
	}

	// B.cond
	if isBCondArm64(word) {
		imm19 := (word >> 5) & 0x7FFFF
		rel := int64(signExtend(imm19, 19) << 2)
		target := int64(originPC) + rel
		newRel := target - int64(trampPC)
		if newRel%4 != 0 {
			return 0, false, fmt.Errorf("arm64 B.cond target not aligned after relocation")
		}
		newImm := newRel >> 2
		if newImm < -(1<<18) || newImm >= (1<<18) {
			return 0, false, fmt.Errorf("arm64 B.cond out of range after relocation: originPC=0x%x trampPC=0x%x target=0x%x",
				originPC, trampPC, uintptr(target))
		}
		return (word &^ 0x00FFFFE0) | ((uint32(newImm) & 0x7FFFF) << 5), true, nil
	}

	// CBZ / CBNZ
	if isCBZArm64(word) || isCBNZArm64(word) {
		imm19 := (word >> 5) & 0x7FFFF
		rel := int64(signExtend(imm19, 19) << 2)
		target := int64(originPC) + rel
		newRel := target - int64(trampPC)
		if newRel%4 != 0 {
			return 0, false, fmt.Errorf("arm64 CBZ/CBNZ target not aligned after relocation")
		}
		newImm := newRel >> 2
		if newImm < -(1<<18) || newImm >= (1<<18) {
			return 0, false, fmt.Errorf("arm64 CBZ/CBNZ out of range after relocation: originPC=0x%x trampPC=0x%x target=0x%x",
				originPC, trampPC, uintptr(target))
		}
		return (word &^ 0x00FFFFE0) | ((uint32(newImm) & 0x7FFFF) << 5), true, nil
	}

	// TBZ / TBNZ
	if isTBZArm64(word) || isTBNZArm64(word) {
		imm14 := (word >> 5) & 0x3FFF
		rel := int64(signExtend(imm14, 14) << 2)
		target := int64(originPC) + rel
		newRel := target - int64(trampPC)
		if newRel%4 != 0 {
			return 0, false, fmt.Errorf("arm64 TBZ/TBNZ target not aligned after relocation")
		}
		newImm := newRel >> 2
		if newImm < -(1<<13) || newImm >= (1<<13) {
			return 0, false, fmt.Errorf("arm64 TBZ/TBNZ out of range after relocation: originPC=0x%x trampPC=0x%x target=0x%x",
				originPC, trampPC, uintptr(target))
		}
		return (word &^ 0x0007FFE0) | ((uint32(newImm) & 0x3FFF) << 5), true, nil
	}

	// ADR
	if isADRArm64(word) {
		imm := decodeImmHiLo21(word) // bytes
		target := int64(originPC) + imm
		newImm := target - int64(trampPC)
		if newImm < -(1<<20) || newImm >= (1<<20) {
			return 0, false, fmt.Errorf("arm64 ADR out of range after relocation: originPC=0x%x trampPC=0x%x target=0x%x",
				originPC, trampPC, uintptr(target))
		}
		return encodeImmHiLo21(word, newImm), true, nil
	}

	// ADRP (page relative)
	if isADRPArm64(word) {
		imm := decodeImmHiLo21(word) << 12 // bytes (page delta)
		originPage := int64(originPC) &^ 0xFFF
		targetPage := originPage + imm

		trampPage := int64(trampPC) &^ 0xFFF
		newPageDelta := (targetPage - trampPage) >> 12
		if (targetPage-trampPage)%4096 != 0 {
			return 0, false, fmt.Errorf("arm64 ADRP page delta not aligned after relocation")
		}
		if newPageDelta < -(1<<20) || newPageDelta >= (1<<20) {
			return 0, false, fmt.Errorf("arm64 ADRP out of range after relocation: originPC=0x%x trampPC=0x%x targetPage=0x%x",
				originPC, trampPC, uintptr(targetPage))
		}
		return encodeImmHiLo21(word, newPageDelta), true, nil
	}

	// LDR (literal) / LDRSW (literal) / PRFM (literal): imm19<<2, base is current PC.
	// Common encodings have op byte: 0x18,0x58,0x98,0xD8 in bits[31:24].
	if isLiteralImm19PCRelArm64(word) {
		imm19 := (word >> 5) & 0x7FFFF
		rel := int64(signExtend(imm19, 19) << 2)
		target := int64(originPC) + rel
		newRel := target - int64(trampPC)
		if newRel%4 != 0 {
			return 0, false, fmt.Errorf("arm64 literal-imm19 target not aligned after relocation")
		}
		newImm := newRel >> 2
		if newImm < -(1<<18) || newImm >= (1<<18) {
			return 0, false, fmt.Errorf("arm64 literal-imm19 out of range after relocation: originPC=0x%x trampPC=0x%x target=0x%x",
				originPC, trampPC, uintptr(target))
		}
		return (word &^ 0x00FFFFE0) | ((uint32(newImm) & 0x7FFFF) << 5), true, nil
	}

	return word, false, nil
}

func signExtend(v uint32, bits uint) int64 {
	shift := 64 - bits
	return (int64(v) << shift) >> shift
}

func decodeImmHiLo21(word uint32) int64 {
	immhi := (word >> 5) & 0x7FFFF
	immlo := (word >> 29) & 0x3
	imm21 := (immhi << 2) | immlo
	return signExtend(imm21, 21)
}

func encodeImmHiLo21(word uint32, imm21 int64) uint32 {
	u := uint32(imm21) & 0x1FFFFF
	immlo := (u & 0x3)
	immhi := (u >> 2) & 0x7FFFF
	// immlo -> bits 30:29, immhi -> bits 23:5
	word = (word &^ 0x60000000) | (immlo << 29)
	word = (word &^ 0x00FFFFE0) | (immhi << 5)
	return word
}

func isBArm64(word uint32) bool  { return (word & 0x7C000000) == 0x14000000 }
func isBLArm64(word uint32) bool { return (word & 0x7C000000) == 0x94000000 }

func isBCondArm64(word uint32) bool { return (word & 0xFF000010) == 0x54000000 }

func isCBZArm64(word uint32) bool  { return (word & 0x7F000000) == 0x34000000 }
func isCBNZArm64(word uint32) bool { return (word & 0x7F000000) == 0x35000000 }

func isTBZArm64(word uint32) bool  { return (word & 0x7F000000) == 0x36000000 }
func isTBNZArm64(word uint32) bool { return (word & 0x7F000000) == 0x37000000 }

func isADRArm64(word uint32) bool  { return (word & 0x9F000000) == 0x10000000 }
func isADRPArm64(word uint32) bool { return (word & 0x9F000000) == 0x90000000 }

func isLiteralImm19PCRelArm64(word uint32) bool {
	switch word & 0xFF000000 {
	case 0x18000000, // LDR (literal) (32-bit)
		0x58000000, // LDR (literal) (64-bit)
		0x98000000, // LDRSW (literal)
		0xD8000000: // PRFM (literal)
		return true
	default:
		return false
	}
}

func isControlFlowPCRelArm64(word uint32) bool {
	return isBArm64(word) || isBLArm64(word) || isBCondArm64(word) ||
		isCBZArm64(word) || isCBNZArm64(word) || isTBZArm64(word) || isTBNZArm64(word)
}

func decodeBranchOffsetArm64(word uint32) (rel int64, ok bool) {
	// returns byte offset relative to current instruction address
	if isBArm64(word) || isBLArm64(word) {
		return signExtend(word&0x03FFFFFF, 26) << 2, true
	}
	if isBCondArm64(word) || isCBZArm64(word) || isCBNZArm64(word) {
		imm19 := (word >> 5) & 0x7FFFF
		return signExtend(imm19, 19) << 2, true
	}
	if isTBZArm64(word) || isTBNZArm64(word) {
		imm14 := (word >> 5) & 0x3FFF
		return signExtend(imm14, 14) << 2, true
	}
	return 0, false
}
