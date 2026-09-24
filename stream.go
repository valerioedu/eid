package main

import (
	"encoding/binary"
)

type InstructionItem struct {
	Address uint64
	Raw     uint32
	Decoded DecodedInst
}

var Data []byte

func getFileFormat() string {
	format := binary.LittleEndian.Uint32(Data[0:4])
	if format == 0xfeedfacf || format == 0xfeedface {
		return getMachFileFormat(Data)
	} else if format == 0x7f454c46 {
		return getELFFileFormat(Data)
	}

	return ""
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
