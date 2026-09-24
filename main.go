package main

import (
	"debug/elf"
	"debug/macho"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
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

func loadBinary(path string) ([]byte, uint64, error) {
	// 1. Try reading as an ELF binary first
	elfFile, err := elf.Open(path)
	machoFile, err2 := macho.Open(path)
	if err == nil {
		defer elfFile.Close()

		if elfFile.Machine != elf.EM_AARCH64 {
			return nil, 0, fmt.Errorf("file is an ELF binary, but not AArch64 (machine: %s)", elfFile.Machine)
		}

		sec := elfFile.Section(".text")
		if sec == nil {
			return nil, 0, fmt.Errorf("ELF has no .text section")
		}

		data, err := sec.Data()
		if err != nil {
			return nil, 0, fmt.Errorf("failed reading .text section: %w", err)
		}
		return data, sec.Addr, nil
	} else if err2 == nil {
		defer machoFile.Close()

		sec := machoFile.Section("__text")
		if sec == nil {
			return nil, 0, fmt.Errorf("mach-o has no __text section")
		}

		data, err := sec.Data()
		if err != nil {
			return nil, 0, fmt.Errorf("failed reading __text section: %s", err)
		}

		sec2 := machoFile.Section("__stubs")
		data2, err := sec2.Data()
		for i := 0; i < len(data2); i++ {
			data = append(data, data2[i])
		}

		return data, sec.Addr, nil
	}

	// 2. Fallback: treat as a flat raw binary file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	return data, 0x0, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <path-to-binary>\n", os.Args[0])
		os.Exit(1)
	}

	data, baseAddr, err := loadBinary(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	Data, err = os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open file")
	}

	stream := DisassembleStream(data, baseAddr)
	if len(stream) == 0 {
		fmt.Println("No executable instructions found.")
		return
	}

	p := tea.NewProgram(UIModel{
		stream: stream,
		cursor: 0,
	}, tea.WithAltScreen()) // Use AltScreen for standard full-screen TUI behavior

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI Error: %v\n", err)
		os.Exit(1)
	}
}
