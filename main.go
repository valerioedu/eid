package main

import "fmt"

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

func main() {

}
