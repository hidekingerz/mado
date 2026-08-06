package ui

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
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

// binaryExts are well-known binary file extensions. Used only to pick
// sidebar colors; opening a file still checks its content with
// looksBinary. SVG is XML text, so it is deliberately absent.
var binaryExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true,
	".webp": true, ".ico": true,
	".mp3": true, ".wav": true, ".flac": true, ".mp4": true, ".mov": true,
	".avi": true,
	".zip": true, ".tar": true, ".gz": true, ".tgz": true, ".bz2": true,
	".xz": true, ".7z": true, ".rar": true,
	".pdf": true,
	".exe": true, ".dll": true, ".so": true, ".dylib": true, ".bin": true,
	".o": true, ".a": true,
	".ttf": true, ".otf": true, ".woff": true, ".woff2": true,
}

// hasBinaryExt reports whether path has a well-known binary extension.
func hasBinaryExt(path string) bool {
	return binaryExts[strings.ToLower(filepath.Ext(path))]
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
