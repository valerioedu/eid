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

	/* Unconditional Branch (register): br, blr, ret, eret, drps, PAC variants (braa, blraa, retaa, etc.)
	 * Format: 1101011 [opc:4] [op2:5] [op3:6] [Rn:5] [op4:5]
	 */
	if extractBits(raw, 31, 25) == 0b1101011 {
		opc := extractBits(raw, 24, 21)
		op2 := extractBits(raw, 20, 16)
		op3 := extractBits(raw, 15, 10)
		rn := extractBits(raw, 9, 5)
		op4 := extractBits(raw, 4, 0)

		if op2 != 0b11111 {
			return unknownInst(raw)
		}

		if op3 == 0b000000 && op4 == 0b00000 {
			switch opc {
			case 0b0000:
				return DecodedInst{
					Mnemonics: "br",
					Operands:  regName(rn, true, false),
				}
			case 0b0001:
				return DecodedInst{
					Mnemonics: "blr",
					Operands:  regName(rn, true, false),
				}
			case 0b0010:
				if rn == 30 {
					return DecodedInst{Mnemonics: "ret"}
				}

				return DecodedInst{
					Mnemonics: "ret",
					Operands:  regName(rn, true, false),
				}
			case 0b0100:
				if rn == 0b11111 {
					return DecodedInst{Mnemonics: "eret"}
				}
			case 0b0101:
				if rn == 0b11111 {
					return DecodedInst{Mnemonics: "drps"}
				}
			}
		}

		if op3 == 0b000010 || op3 == 0b000011 {
			key := "a"
			if (op3 & 1) == 1 {
				key = "b"
			}

			switch opc {
			case 0b0000:
				if op4 == 0b11111 {
					return DecodedInst{
						Mnemonics: "bra" + key + "z",
						Operands:  regName(rn, true, false),
					}
				}
			case 0b0001:
				if op4 == 0b11111 {
					return DecodedInst{
						Mnemonics: "blra" + key + "z",
						Operands:  regName(rn, true, false),
					}
				}
			case 0b0010:
				if rn == 0b11111 && op4 == 0b11111 {
					return DecodedInst{Mnemonics: "reta" + key}
				}
			case 0b0100:
				if rn == 0b11111 && op4 == 0b11111 {
					return DecodedInst{Mnemonics: "ereta" + key}
				}
			case 0b1000:
				return DecodedInst{
					Mnemonics: "bra" + key,
					Operands:  fmt.Sprintf("%s, %s", regName(rn, true, false), regName(op4, true, true)),
				}
			case 0b1001:
				return DecodedInst{
					Mnemonics: "blra" + key,
					Operands:  fmt.Sprintf("%s, %s", regName(rn, true, false), regName(op4, true, true)),
				}
			}
		}

		return unknownInst(raw)
	}

	/* Test and Branch (immediate): op=0 -> TBZ, op=1 -> TBNZ
	 * Format: [b5:1] 011011 [op:1] [b40:5] [imm14:14] [Rt:5]
	 */
	if extractBits(raw, 30, 25) == 0b011011 {
		b5 := extractBits(raw, 31, 31)
		op := extractBits(raw, 24, 24)
		b40 := extractBits(raw, 23, 19)
		imm14 := extractBits(raw, 18, 5)
		rt := extractBits(raw, 4, 0)

		mnemonic := "tbz"
		if op == 1 {
			mnemonic = "tbnz"
		}

		bitPos := (b5 << 5) | b40
		offset := signExtend(imm14, 14) * 4
		target := uint64(int64(pc) + offset)

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  fmt.Sprintf("%s, #%d, 0x%x", regName(rt, b5 == 1, false), bitPos, target),
			Target:    target,
		}
	}

	/* Add/Sub
	 * Format: [sf:1] [op:1] [S:1] 01011 [shift:2] 0 [Rm:5] [imm6:6] [Rn:5] [Rd:5]
	 */
	if extractBits(raw, 28, 24) == 0b01011 && extractBits(raw, 21, 21) == 0 {
		shift := extractBits(raw, 23, 22)
		imm6 := extractBits(raw, 15, 10)
		sf := extractBits(raw, 31, 31) == 1

		if shift == 0b11 || (!sf && imm6 >= 32) {
			return unknownInst(raw)
		}

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

		operands := fmt.Sprintf("%s, %s, %s", regName(rd, sf, false), regName(rn, sf, false), regName(rm, sf, false))
		if imm6 > 0 {
			shiftNames := []string{"lsl", "lsr", "asr"}
			operands += fmt.Sprintf(", %s #%d", shiftNames[shift], imm6)
		}

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  operands,
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

		rdStr := regName(rd, sf, s == 0)
		rnStr := regName(rn, sf, true)

		if op == 1 && s == 1 && rd == 31 {
			return DecodedInst{
				Mnemonics: "cmp",
				Operands:  fmt.Sprintf("%s, #0x%x", rnStr, imm12),
			}
		} else if op == 0 && s == 0 && imm12 == 0 && (rd == 31 || rn == 31) {
			return DecodedInst{
				Mnemonics: "mov",
				Operands:  fmt.Sprintf("%s, %s", rdStr, rnStr),
			}
		}

		mnemonic := "add"
		if op == 1 {
			mnemonic = "sub"
		}

		if s == 1 {
			mnemonic += "s"
		}

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
		v := extractBits(raw, 26, 26)
		l := extractBits(raw, 22, 22)
		imm7 := extractBits(raw, 21, 15)
		rt2 := extractBits(raw, 14, 10)
		rn := extractBits(raw, 9, 5)
		rt1 := extractBits(raw, 4, 0)

		if (v == 0 && (opc&1) != 0) || (v == 1 && opc == 3) {
			return unknownInst(raw)
		}

		mnemonic := "stp"
		if l == 1 {
			mnemonic = "ldp"
		}

		var shift uint
		var rt1Str, rt2Str string
		rnStr := regName(rn, true, true)

		if v == 0 {
			is64Bit := opc == 2
			shift = 2
			if is64Bit {
				shift = 3
			}
			rt1Str = regName(rt1, is64Bit, false)
			rt2Str = regName(rt2, is64Bit, false)
		} else {
			shift = uint(2 + opc)
			rt1Str = regNameSIMD(rt1, opc+2)
			rt2Str = regNameSIMD(rt2, opc+2)
		}

		offset := signExtend(imm7, 7) << shift
		var opStr string
		if offset < 0 {
			opStr = fmt.Sprintf("%s, %s, [%s, #-0x%x]", rt1Str, rt2Str, rnStr, -offset)
		} else if offset > 0 {
			opStr = fmt.Sprintf("%s, %s, [%s, #0x%x]", rt1Str, rt2Str, rnStr, offset)
		} else {
			opStr = fmt.Sprintf("%s, %s, [%s]", rt1Str, rt2Str, rnStr)
		}

		return DecodedInst{
			Mnemonics: mnemonic,
			Operands:  opStr,
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

		if !sf && hw >= 32 {
			return unknownInst(raw)
		}

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

	/* No operation (nop)
	 * Format: 11010101 00000011 00100000 00011111
	 */
	if raw == 0b11010101000000110010000000011111 {
		return DecodedInst{
			Mnemonics: "nop",
		}
	}

	/* Exception Generation: SVC, BRK
	 * Format: 11010100 [opc:3] [imm16:16] [op2:3] [ll:2]
	 */
	if extractBits(raw, 31, 24) == 0b11010100 {
		opc := extractBits(raw, 23, 21)
		imm16 := extractBits(raw, 20, 5)
		ll := extractBits(raw, 1, 0)

		if extractBits(raw, 4, 2) != 0b000 {
			return unknownInst(raw)
		}

		if opc == 0 && ll == 1 {
			return DecodedInst{
				Mnemonics: "svc",
				Operands:  fmt.Sprintf("#0x%x", imm16),
			}
		}

		if opc == 1 && ll == 0 {
			return DecodedInst{
				Mnemonics: "brk",
				Operands:  fmt.Sprintf("#0x%x", imm16),
			}
		}
	}

	/*
	 * Format: 110101010000001100 [opc:2] [CRm:4] [op2:3] [Rd:5]
	 */
	if extractBits(raw, 31, 14) == 0b110101010000001100 {
		opc := extractBits(raw, 13, 12)
		crm := extractBits(raw, 11, 8)
		op2 := extractBits(raw, 7, 5)
		rd := extractBits(raw, 4, 0)

		if opc == 0b01 && crm == 0b0000 {
			switch op2 {
			case 0b000:
				return DecodedInst{
					Mnemonics: "wfet",
					Operands:  regName(rd, true, false),
				}
			case 0b001:
				return DecodedInst{
					Mnemonics: "wfit",
					Operands:  regName(rd, true, false),
				}
			}
		} else if opc == 0b10 {
			if crm != 0b0000 || rd != 0b11111 {
				return unknownInst(raw)
			}

			switch op2 {
			case 0b010:
				return DecodedInst{
					Mnemonics: "wfe",
				}
			case 0b011:
				return DecodedInst{
					Mnemonics: "wfi",
				}
			case 0b001:
				return DecodedInst{
					Mnemonics: "yield",
				}
			}
		}

		return unknownInst(raw)
	}

	return unknownInst(raw)
}

func unknownInst(raw uint32) DecodedInst {
	return DecodedInst{
		Mnemonics: ".word",
		Operands:  fmt.Sprintf("0x%08x", raw),
	}
}
