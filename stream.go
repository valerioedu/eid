package main

import (
	"encoding/binary"
)

type InstructionItem struct {
	Address uint64
	Raw     uint32
	Decoded DecodedInst
}

func DisassembleStream(data []byte, baseAddr uint64) []InstructionItem {
	var program []InstructionItem
	for i := 0; i <= len(data)-4; i += 4 {
		addr := baseAddr + uint64(i)
		raw := binary.LittleEndian.Uint32(data[i : i+4])

		program = append(program, InstructionItem{
			Address: addr,
			Raw:     raw,
			Decoded: DecodeAArch64(raw, addr),
		})
	}

	return program
}
