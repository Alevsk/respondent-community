package declarative

import (
	"fmt"
)

// decodeLZW decompresses an LZW-encoded byte slice into a UTF-8 string.
// This implements the JavaScript-style LZW variant used by Blitzortung,
// which operates on UTF-16 code points (not GIF/TIFF LZW).
func decodeLZW(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("lzw: empty input")
	}

	// Convert raw bytes to slice of integer code points.
	// Messages from Blitzortung arrive as UTF-8 text containing characters
	// whose Unicode code points are the LZW dictionary indices.
	runes := []rune(string(data))
	if len(runes) == 0 {
		return nil, fmt.Errorf("lzw: no code points")
	}

	// Initialize dictionary with single-character entries (0..255).
	dictSize := 256
	dict := make(map[int]string, 512)
	for i := 0; i < 256; i++ {
		dict[i] = string(rune(i))
	}

	w := dict[int(runes[0])]
	result := []byte(w)

	for i := 1; i < len(runes); i++ {
		code := int(runes[i])

		var entry string
		if s, ok := dict[code]; ok {
			entry = s
		} else if code == dictSize {
			// Special case: code not yet in dictionary.
			entry = w + string(w[0])
		} else {
			return nil, fmt.Errorf("lzw: invalid code %d at position %d (dict size %d)", code, i, dictSize)
		}

		result = append(result, entry...)

		// Add w + entry[0] to dictionary.
		dict[dictSize] = w + string(entry[0])
		dictSize++

		w = entry
	}

	return result, nil
}
