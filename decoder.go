package main

import (
	"fmt"
)

type DecodedInst struct {
	Mnemonics string
	Operands  string
	Target    uint64 // For branch target
}

var condNames = []string{
	"eq", "ne", "cs", "cc", "mi", "pl", "vs", "vc",
	"hi", "ls", "ge", "lt", "gt", "le", "al", "nv",
}

func DecodeAArch64(raw uint32, pc uint64) DecodedInst {
	/* Unconditional Branch (b, bl): op=0 -> B, op=1 -> BL
	 *Format: [op:1] 00101 [imm26:26]
	 */
	if extractBits(raw, 30, 26) == 0b00101 {
		op := extractBits(raw, 31, 31)
		imm26 := extractBits(raw, 25, 0)
		offset := signExtend(imm26, 26) * 4 // 4B per instruction
		target := uint64(int64(pc) + offset)

		mnemonic := "b"
		if op == 1 {
			mnemonic = "bl"
		}

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("0x%x", target),
			Target:    target,
		}
	}

	/* Conditional Branch (immediate): b.<cond>
	 * Format: 01010100 [imm19:19] 0 [cond:4]
	 */
	if extractBits(raw, 31, 24) == 0b01010100 && extractBits(raw, 4, 4) == 0 {
		imm19 := extractBits(raw, 23, 5)
		cond := extractBits(raw, 3, 0)
		offset := signExtend(imm19, 19) * 4
		target := uint64(int64(pc) + offset)

		return DecodedInst{
			Mnemonics: fmt.Sprintf("b.%s", condNames[cond]),
			Operands:  fmt.Sprintf("0x%x", target),
			Target:    target,
		}
	}

	/* Compare and Branch (immediate): op=0 -> CBZ, op=1 -> CBNZ
	 * Format: [sf:1] 011010 [op:1] [imm19:19] [Rt:5]
	 */
	if extractBits(raw, 30, 25) == 0b011010 {
		sf := extractBits(raw, 31, 31) == 1
		op := extractBits(raw, 24, 24)
		imm19 := extractBits(raw, 23, 5)
		rt := extractBits(raw, 4, 0)

		offset := signExtend(imm19, 19) * 4
		target := uint64(int64(pc) + offset)

		mnemonic := "cbz"
		if op == 1 {
			mnemonic = "cbnz"
		}

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("%s, 0x%x", regName(rt, sf, false), target),
			Target:    target,
		}
	}

	/* Branch Register (br)
	 * Format: 11010110 [Rn:5]
	 */
	if extractBits(raw, 31, 24) == 0b11010110 {
		opc := extractBits(raw, 24, 21)
		if opc == 0 || opc == 1 {
			var mnemonic string
			if opc == 0 {
				mnemonic = "br"
			} else {
				mnemonic = "blr"
			}

			rn := extractBits(raw, 9, 5)

			return DecodedInst{
				Mnemonics: mnemonic,
				Operands:  fmt.Sprintf("%s", regName(rn, true, false)),
			}
		}
	}

	/* Add/Sub
	 * Format: [sf:1] [op:1] [S:1] 01011 [shift:2] 0 [Rm:5] [imm6:6] [Rn:5] [Rd:5]
	 */
	if extractBits(raw, 28, 24) == 0b01011 && extractBits(raw, 21, 21) == 0 {
		sf := extractBits(raw, 31, 31) == 1
		op := extractBits(raw, 30, 30)
		s := extractBits(raw, 29, 29)
		rm := extractBits(raw, 20, 16)
		rn := extractBits(raw, 9, 5)
		rd := extractBits(raw, 4, 0)

		mnemonic := "add"
		if op == 1 {
			mnemonic = "sub"
		}

		if s == 1 {
			mnemonic += "s"
		}

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("%s, %s, %s", regName(rd, sf, true), regName(rn, sf, true), regName(rm, sf, false)),
		}
	}

	/* Add/Sub (immediate)
	 * Format: [sf:1] [op:1] [S:1] 100010 [sh:1] [imm12:12] [Rn:5] [Rd:5]
	 */
	if extractBits(raw, 28, 23) == 0b100010 {
		sf := extractBits(raw, 31, 31) == 1
		op := extractBits(raw, 30, 30)
		s := extractBits(raw, 29, 29)
		shift := extractBits(raw, 22, 22) * 12
		imm12 := extractBits(raw, 21, 10) << shift
		rn := extractBits(raw, 9, 5)
		rd := extractBits(raw, 4, 0)

		mnemonic := "add"
		if op == 1 {
			mnemonic = "sub"
		}

		if s == 1 {
			mnemonic += "s"
		}

		rdStr := regName(rd, sf, true)
		rnStr := regName(rn, sf, true)
		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("%s, %s, #0x%x", rdStr, rnStr, imm12),
		}
	}

	/* PC-Relative Addressing: ADR, ADRP
	 * Format: [op:1] [immlo:2] 10000 [immhi:19] [Rd:5]
	 */
	if extractBits(raw, 28, 24) == 0b10000 {
		op := extractBits(raw, 31, 31)
		immlo := extractBits(raw, 30, 29)
		immhi := extractBits(raw, 23, 5)
		rd := extractBits(raw, 4, 0)

		imm := (immhi << 2) | immlo
		var target uint64
		mnemonic := "adr"

		if op == 1 {
			mnemonic = "adrp"
			target = (pc &^ 0xfff) + uint64(signExtend(imm, 21)<<12)
		} else {
			target = uint64(int64(pc) + signExtend(imm, 21))
		}

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("%s, 0x%x", regName(rd, true, false), target),
			Target:    target,
		}
	}

	/* Load/Store Register (unsigned immediate): opc=0 -> STR, opc=1 -> LDR
	 * Format: [size:2] 111001 [opc:2] [imm12:12] [Rn:5] [Rt:5]
	 */
	if extractBits(raw, 29, 24) == 0b111001 {
		size := extractBits(raw, 31, 30)
		opc := extractBits(raw, 23, 22)
		imm12 := extractBits(raw, 21, 10)
		rn := extractBits(raw, 9, 5)
		rt := extractBits(raw, 4, 0)

		if size >= 2 && opc <= 1 {
			is64Bit := (size == 3)
			offset := imm12 << size

			mnemonic := "str"
			if opc == 1 {
				mnemonic = "ldr"
			}

			rtStr := regName(rt, is64Bit, false)
			rnStr := regName(rn, true, true)

			var opStr string
			if offset > 0 {
				opStr = fmt.Sprintf("%s, [%s, #0x%x]", rtStr, rnStr, offset)
			} else {
				opStr = fmt.Sprintf("%s, [%s]", rtStr, rnStr)
			}

			return DecodedInst{
				Mnemonics: mnemonic,
				Operands:  opStr,
			}
		}
	}

	/* Load/Store Register Pair (Offset): STP, LDP
	 * Format: [opc:2] 101 0 010 [L:1] [imm7:7] [Rt2:5] [Rn:5] [Rt1:5]
	 */
	if extractBits(raw, 29, 27) == 0b101 && extractBits(raw, 25, 23) == 0b010 {
		opc := extractBits(raw, 31, 30)
		l := extractBits(raw, 22, 22)
		imm7 := extractBits(raw, 21, 15)
		rt2 := extractBits(raw, 14, 10)
		rn := extractBits(raw, 9, 5)
		rt1 := extractBits(raw, 4, 0)

		is64Bit := opc == 2
		mnemonic := "stp"
		if l == 1 {
			mnemonic = "ldp"
		}

		shift := uint(2)
		if is64Bit {
			shift = 3
		}
		offset := signExtend(imm7, 7) << shift

		rt1Str := regName(rt1, is64Bit, false)
		rt2Str := regName(rt2, is64Bit, false)
		rnStr := regName(rn, true, true)

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("%s, %s, [%s, #0x%x]", rt1Str, rt2Str, rnStr, offset),
		}
	}

	/* Move Wide Immediate (movz, movk, movn)
	 * Format: [sf:1] [opc:2] 100101 [hw:2] [imm16:16] [Rd:5]
	 */
	if extractBits(raw, 28, 23) == 0b100101 {
		sf := extractBits(raw, 31, 31) == 1
		opc := extractBits(raw, 30, 29)
		hw := extractBits(raw, 22, 21) * 16
		imm16 := extractBits(raw, 20, 5)
		rd := extractBits(raw, 4, 0)

		var mnemonic string
		switch opc {
		case 0b00:
			mnemonic = "movn"
		case 0b10:
			mnemonic = "movz"
		case 0b11:
			mnemonic = "movk"
		default:
			return unknownInst(raw)
		}

		rdStr := regName(rd, sf, false)
		if hw > 0 {
			return DecodedInst{
				Mnemonics: mnemonic,
				Operands:  fmt.Sprintf("%s, #0x%x, lsl #%d", rdStr, imm16, hw),
			}
		}

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("%s, #0x%x", rdStr, imm16),
		}
	}

	/* Return (ret)
	 * Format: 1101011 0 0 10 11111 000000 [Rn:5] 00000
	 */
	if raw&0xfffffc1f == 0xd65f0000 {
		rn := extractBits(raw, 9, 5)
		if rn == 30 {
			return DecodedInst{
				Mnemonics: "ret",
				Operands:  "",
			}
		}

		return DecodedInst{
			Mnemonics: "ret",
			Operands:  regName(rn, true, false),
		}
	}

	return unknownInst(raw)
}

func unknownInst(raw uint32) DecodedInst {
	return DecodedInst{
		Mnemonics: ".word",
		Operands:  fmt.Sprintf("0x%08x", raw),
	}
}
