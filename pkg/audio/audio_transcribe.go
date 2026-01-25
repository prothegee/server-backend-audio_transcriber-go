package pkg_audio

import (
	"context"
)

// transcribeRequest represents a request to transcribe audio
type TranscribeRequest struct {
	Audio     []byte
	Resp      chan<- *TranscribeResult
	Ctx       context.Context
	SessionID string
}

// transcribeResult is the result of transcription
type TranscribeResult struct {
	Text     string
	Warning  bool
	Keywords []string
	Err      error
}
