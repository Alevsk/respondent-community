package declarative

import (
	"encoding/json"
	"testing"
)

func TestDecodeLZW_empty(t *testing.T) {
	_, err := decodeLZW(nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
	_, err = decodeLZW([]byte{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestDecodeLZW_roundtrip(t *testing.T) {
	// LZW-encode a known JSON string using the same algorithm.
	input := `{"lat":35.72,"lon":-85.26,"time":1773644575219495700,"alt":0,"pol":0,"mds":11400,"mcg":96}`

	encoded := encodeLZW(input)
	decoded, err := decodeLZW(encoded)
	if err != nil {
		t.Fatalf("decodeLZW error: %v", err)
	}

	if string(decoded) != input {
		t.Errorf("roundtrip mismatch:\n  got:  %s\n  want: %s", decoded, input)
	}

	// Verify decoded output is valid JSON.
	var m map[string]interface{}
	if err := json.Unmarshal(decoded, &m); err != nil {
		t.Errorf("decoded output is not valid JSON: %v", err)
	}
}

// encodeLZW is the JS-style LZW encoder (inverse of decodeLZW).
func encodeLZW(input string) []byte {
	dict := make(map[string]int, 512)
	for i := 0; i < 256; i++ {
		dict[string(rune(i))] = i
	}
	dictSize := 256

	w := ""
	var result []rune

	for _, c := range input {
		wc := w + string(c)
		if _, ok := dict[wc]; ok {
			w = wc
		} else {
			result = append(result, rune(dict[w]))
			dict[wc] = dictSize
			dictSize++
			w = string(c)
		}
	}

	if w != "" {
		result = append(result, rune(dict[w]))
	}

	return []byte(string(result))
}
