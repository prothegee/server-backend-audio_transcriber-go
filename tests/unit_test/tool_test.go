package unit_test

import (
	"testing"

	"showcase-backend-audio_transcriber-go/pkg"
)

func TestBytesToFloat32(t *testing.T) {
	tests := []struct {
		name    string
		input   []byte
		wantErr bool
	}{
		{
			name:    "even length",
			input:   []byte{0x00, 0x00, 0x00, 0x80},
			wantErr: false,
		},
		{
			name:    "odd length",
			input:   []byte{0x00, 0x00, 0x01},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := pkg.BytesToFloat32(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("BytesToFloat32() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestContainsKeywords(t *testing.T) {
	text := "This is a secret message with forbidden words like bomb and kill."
	keywords := []string{"bomb", "kill", "weapon"}

	has, found := pkg.ContainsKeywords(text, keywords)
	if !has {
		t.Errorf("expected to find keywords, got none")
	}
	if len(found) != 2 {
		t.Errorf("expected 2 keywords, got %d", len(found))
	}

	has, _ = pkg.ContainsKeywords("BOMB in caps", []string{"bomb"})
	if !has {
		t.Errorf("case-insensitive match failed")
	}
}

func TestInt16SliceToBytes(t *testing.T) {
	input := []int16{0, 1, -1, 32767, -32768}
	expected := []byte{
		0x00, 0x00,
		0x01, 0x00,
		0xFF, 0xFF,
		0xFF, 0x7F,
		0x00, 0x80,
	}
	result := pkg.Int16SliceToBytes(input)
	if len(result) != len(expected) {
		t.Fatalf("length mismatch: got %d, want %d", len(result), len(expected))
	}
	for i := range result {
		if result[i] != expected[i] {
			t.Errorf("byte %d: got 0x%02X, want 0x%02X", i, result[i], expected[i])
		}
	}
}
