package ui

import (
	"bytes"
	"fmt"
)

// binarySampleSize is how many leading bytes are inspected when deciding
// whether a file is binary.
const binarySampleSize = 8192

// looksBinary reports whether data appears to be a binary file. A NUL
// byte in the leading sample is the signal: real text never contains
// one, while virtually every binary format (PNG, JPEG, zip, ELF, …)
// does within the first few KB.
func looksBinary(data []byte) bool {
	sample := data
	if len(sample) > binarySampleSize {
		sample = sample[:binarySampleSize]
	}
	return bytes.IndexByte(sample, 0) >= 0
}

// humanSize formats a byte count for the binary-file placeholder.
func humanSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
