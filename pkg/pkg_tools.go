package pkg

import (
	"fmt"
	"strings"
)

func BytesToFloat32(data []byte) ([]float32, error) {
	if len(data)%2 != 0 {
		return nil, fmt.Errorf("data length must be even for 16-bit audio")
	}
	floats := make([]float32, len(data)/2)
	for i := range floats {
		sample := int16(data[i*2]) | int16(data[i*2+1])<<8
		floats[i] = float32(sample) / 32768.0
	}
	return floats, nil
}

func ContainsKeywords(text string, keywords []string) (bool, []string) {
	found := []string{}
	lowerText := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			found = append(found, kw)
		}
	}
	return len(found) > 0, found
}

func Int16SliceToBytes(data []int16) []byte {
    bytes := make([]byte, len(data)*2)
    for i, v := range data {
        bytes[i*2] = byte(v)
        bytes[i*2+1] = byte(v >> 8)
    }
    return bytes
}
