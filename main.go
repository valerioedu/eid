package main

import (
	"debug/elf"
	"debug/macho"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

type DataSection struct {
	Name    string
	Address uint64
	Data    []byte
}

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

func loadBinary(path string) ([]byte, uint64, string, []DataSection, error) {
	// Try reading as an ELF/mach-o binary first
	if elfFile, err := elf.Open(path); err == nil {
		defer elfFile.Close()

		if elfFile.Machine != elf.EM_AARCH64 {
			return nil, 0, "", nil, fmt.Errorf("file is an ELF binary, but not AArch64 (machine: %s)", elfFile.Machine)
		}

		arch := elfFile.Machine.String()
		fileType := elfFile.Type.String()
		class := elfFile.Class.String()
		meta := fmt.Sprintf("ELF (%s) | %s | %s", class, arch, fileType)

		sec := elfFile.Section(".text")
		if sec == nil {
			return nil, 0, "", nil, fmt.Errorf("ELF has no .text section")
		}

		data, err := sec.Data()
		if err != nil {
			return nil, 0, "", nil, fmt.Errorf("failed reading .text section: %w", err)
		}

		var dataSecs []DataSection
		for _, s := range elfFile.Sections {
			if s.Name == ".rodata" || s.Name == ".data" || s.Name == ".bss" {
				d, _ := s.Data()
				dataSecs = append(dataSecs, DataSection{Name: s.Name, Address: s.Addr, Data: d})
			}
		}

		return data, sec.Addr, meta, dataSecs, nil
	}

	if machoFile, err := macho.Open(path); err == nil {
		defer machoFile.Close()

		sec := machoFile.Section("__text")
		if sec == nil {
			return nil, 0, "", nil, fmt.Errorf("mach-o has no __text section")
		}

		data, err := sec.Data()
		if err != nil {
			return nil, 0, "", nil, fmt.Errorf("failed reading __text section: %s", err)
		}

		var arch string
		switch machoFile.Cpu {
		case macho.CpuArm64:
			arch = "arm64"
		case macho.CpuArm:
			arch = "arm"
		case macho.CpuAmd64:
			arch = "AMD64"
		case macho.Cpu386:
			arch = "x86"
		default:
			arch = machoFile.Cpu.String()
		}

		var fileType string
		switch machoFile.Type {
		case macho.TypeExec:
			fileType = "Executable"
		case macho.TypeDylib:
			fileType = "Dynamic Library"
		case macho.TypeObj:
			fileType = "Object File"
		case macho.TypeBundle:
			fileType = "Bundle"
		}

		meta := fmt.Sprintf("mach-o | %s | %s", arch, fileType)
		var dataSecs []DataSection
		for _, s := range machoFile.Sections {
			if s.Name == "__cstring" || s.Name == "__const" || s.Name == "__data" {
				d, _ := s.Data()
				dataSecs = append(dataSecs, DataSection{Name: s.Name, Address: s.Addr, Data: d})
			}
		}

		return data, sec.Addr, meta, dataSecs, nil
	}

	// Fallback: treat as a flat raw binary file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, "raw", nil, err
	}

	return data, 0x0, "raw", nil, nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <path-to-binary>\n", os.Args[0])
		os.Exit(1)
	}

	data, baseAddr, format, dataSecs, err := loadBinary(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading file: %v\n", err)
		os.Exit(1)
	}

	stream := DisassembleStream(data, baseAddr)
	if len(stream) == 0 {
		fmt.Println("No executable instructions found.")
		return
	}

	p := tea.NewProgram(UIModel{
		format:     format,
		stream:     stream,
		cursor:     0,
		viewState:  ViewText,
		dataSecs:   dataSecs,
		dataCursor: 0,
	}, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TUI Error: %v\n", err)
		os.Exit(1)
	}
}
