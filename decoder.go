package main

import (
	"fmt"
)

type DecodedInst struct {
	Mnemonics string
	Operands  string
	Target    uint64 // For branch target
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

	/* Add/Sub (immediate)
	 * Format: [sf:1] [op:1] [S:1] 100010 [sh:1] [imm12:12] [Rn:5] [Rd:5]
	 */
	if extractBits(raw, 28, 23) == 0b100010 {
		sf := extractBits(raw, 31, 31) == 1
		op := extractBits(raw, 30, 30)
		s := extractBits(raw, 29, 29)
		shift := extractBits(raw, 22, 22) * 16
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

	/* Move Wide Immediate (movz, movk, movn)
	 * Format: [sf:1] [opc:2] 100101 [hw:2] [imm16:16] [Rd:5]
	 */
	if extractBits(raw, 28, 23) == 0b100101 {
		sf := extractBits(raw, 31, 31) == 1
		opc := extractBits(raw, 30, 29)
		hw := extractBits(raw, 22, 21) * 12
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
	 *Format: 1101011 0 0 10 11111 000000 [Rn:5] 00000
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
