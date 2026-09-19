package main

import (
	"fmt"
)

func extractBits(inst uint32, high, low uint) uint32 {
	mask := (uint32(1) << (high - low + 1)) - 1
	return (inst >> low) & mask
}

func signExtend(val uint32, bits uint) int64 {
	shift := 64 - bits
	return (int64(val) << shift) >> shift
}

func regName(reg uint32, is64Bit bool, isSP bool) string {
	if reg == 31 {
		if isSP {
			return "sp"
		} else if is64Bit {
			return "xzr"
		} else {
			return "wzr"
		}
	} else if reg == 30 {
		return "lr"
	} else if reg == 29 {
		return "fp"
	}

	prefix := "w"
	if is64Bit {
		prefix = "x"
	}

	return fmt.Sprintf("%s%d", prefix, reg)
}

// TestBinary contains 12 instructions (48 bytes) representing:
//
// 0x400000: sub  sp, sp, #0x20       ; Allocate stack frame
// 0x400004: movz x0, #0x1337        ; Load lower 16 bits of immediate
// 0x400008: movk x0, #0x42, lsl #16 ; Insert next 16 bits
// 0x40000c: movz x1, #0xa           ; Load loop/counter value
// 0x400010: add  x0, x0, #0x1       ; Increment x0
// 0x400014: subs w1, w1, #0x1       ; Decrement w1 and set flags
// 0x400018: b    0x400024           ; Jump forward over unreachable code
// 0x40001c: movz x2, #0xdead        ; [Skipped target]
// 0x400020: add  x0, x0, #0xff      ; [Skipped target]
// 0x400024: add  sp, sp, #0x20       ; [Branch Target] Restore stack pointer
// 0x400028: bl   0x40002c           ; Call next instruction (sets link register)
// 0x40002c: ret                     ; Return
var TestBinary = []byte{
	// 0x400000: sub sp, sp, #0x20 (d10083ff)
	0xff, 0x83, 0x00, 0xd1,

	// 0x400004: movz x0, #0x1337 (d28266e0)
	0xe0, 0x66, 0x82, 0xd2,

	// 0x400008: movk x0, #0x42, lsl #16 (f2a00840)
	0x40, 0x08, 0xa0, 0xf2,

	// 0x40000c: movz x1, #0xa (d2800141)
	0x41, 0x01, 0x80, 0xd2,

	// 0x400010: add x0, x0, #0x1 (91000400)
	0x00, 0x04, 0x00, 0x91,

	// 0x400014: subs w1, w1, #0x1 (71000421)
	0x21, 0x04, 0x00, 0x71,

	// 0x400018: b 0x400024 (+12 bytes / 3 instructions) (14000003)
	0x03, 0x00, 0x00, 0x14,

	// 0x40001c: movz x2, #0xdead (d29bd5a2)
	0xa2, 0xd5, 0x9b, 0xd2,

	// 0x400020: add x0, x0, #0xff (9103fc00)
	0x00, 0xfc, 0x03, 0x91,

	// 0x400024: add sp, sp, #0x20 (910083ff)
	0xff, 0x83, 0x00, 0x91,

	// 0x400028: bl 0x40002c (+4 bytes) (94000001)
	0x01, 0x00, 0x00, 0x94,

	// 0x40002c: ret (d65f03c0)
	0xc0, 0x03, 0x5f, 0xd6,
}

const BaseAddress = uint64(0x00400000)

func main() {
	stream := DisassembleStream(TestBinary, BaseAddress)

	fmt.Printf("%-10s  %-8s  %-8s %s\n", "ADDRESS", "BYTES", "MNEMONIC", "OPERANDS")
	fmt.Println("--------------------------------------------------")

	for _, item := range stream {
		fmt.Printf("0x%08x  %08x  %-8s %s\n",
			item.Address,
			item.Raw,
			item.Decoded.Mnemonics,
			item.Decoded.Operands,
		)
	}
}
