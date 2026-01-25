package main

import (
	"sync"
	"testing"

	pkg "showcase-backend-audio_transcriber-go/pkg"
)

func TestAudioAccumulationAndSending(t *testing.T) {
	sampleRate = 16000
	audioBufferChannelSize = 10

	audioChan := make(chan []int16, audioBufferChannelSize)
	bytesPerSecond := int(sampleRate * 2) // 32000 bytes/sec

	var wg sync.WaitGroup
	wg.Add(1)

	// sender goroutine
	go func() {
		defer wg.Done()
		defer close(audioChan) // penting: tutup channel setelah selesai

		halfSec := make([]int16, 8000) // 0.5 detik @ 16kHz
		for i := range halfSec {
			halfSec[i] = int16(i % 1000)
		}

		// kirim 3 chunk (total 1.5 detik)
		for i := 0; i < 3; i++ {
			audioChan <- halfSec
		}
	}()

	// tunggu sampai semua data dikirim dan channel ditutup
	wg.Wait()

	// kumpulkan semua data
	var sendBuffer []byte
	for samples := range audioChan {
		bytes := pkg.Int16SliceToBytes(samples)
		sendBuffer = append(sendBuffer, bytes...)
	}

	if len(sendBuffer) < bytesPerSecond {
		t.Errorf("sendBuffer too small: %d bytes, expected >= %d", len(sendBuffer), bytesPerSecond)
	}
	if len(sendBuffer) != 3*8000*2 {
		t.Errorf("expected 48000 bytes, got %d", len(sendBuffer))
	}
}
