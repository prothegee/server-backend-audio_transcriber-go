package pkg_whisper

import (
	"sync"
	"log"
	"fmt"
	"strings"

	pkg "showcase-backend-audio_transcriber-go/pkg"
	pkg_audio "showcase-backend-audio_transcriber-go/pkg/audio"
	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
)

// whisperWorkerPool initializes a pool of workers to process requests concurrently
// we pass a *sync.Mutex to ensure only one inference runs at a time, prevent external lib SIGSEGV
func WhisperWorkerPool(model whisper.Model, reqChan <-chan *pkg_audio.TranscribeRequest, numWorkers int, inferenceMu *sync.Mutex, fbdkwrds []string) {
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			log.Printf("worker #%d started", workerID)

			// context per worker
			ctx, err := model.NewContext()
			if err != nil {
				log.Fatalf("worker #%d failed to create whisper context: %v", workerID, err)
			}

			ctx.SetLanguage("auto")
			ctx.SetTranslate(false)

			for req := range reqChan {
				select {
				case <-req.Ctx.Done():
					log.Printf("[worker #%d] context cancelled for session %s", workerID, req.SessionID)
					continue
				default:
					// proceed
				}

				// cpu bound: convert bytes to floats
				// - parallel safe
				// - do outside the lock to maximize concurrency
				audioFloats, err := pkg.BytesToFloat32(req.Audio)
				if err != nil {
					select {
					case req.Resp <- &pkg_audio.TranscribeResult{Err: fmt.Errorf("audio conversion: %w", err)}:
					default:
						log.Printf("[worker #%d] resp chan full for session %s", workerID, req.SessionID)
					}
					continue
				}

				var result strings.Builder
				segmentCallback := func(segment whisper.Segment) {
					result.WriteString(segment.Text)
				}

				// cgo bound: inference (critical)
				// - gglm whisper is not thread safe
				// - concurrent process calls on the same backend state
				inferenceMu.Lock()
				err = ctx.Process(audioFloats, nil, segmentCallback, nil)
				inferenceMu.Unlock()

				if err != nil {
					select {
					case req.Resp <- &pkg_audio.TranscribeResult{Err: fmt.Errorf("whisper process: %w", err)}:
					default:
						log.Printf("[worker #%d] resp chan full for session %s", workerID, req.SessionID)
					}
					continue
				}

				text := strings.TrimSpace(result.String())
				
				if text == "" || text == "BLANK_AUDIO" || len(text) < 2 {
					select {
					case req.Resp <- &pkg_audio.TranscribeResult{}:
					default:
					}
					continue
				}

				hasKeywords, found := pkg.ContainsKeywords(text, fbdkwrds)
				res := &pkg_audio.TranscribeResult{
					Text:     text,
					Warning:  hasKeywords,
					Keywords: found,
				}

				// send result, if fail just log
				select {
				case req.Resp <- res:
				default:
					log.Printf("[worker #%d] failed to send result (chan full) for session %s", workerID, req.SessionID)
				}
			}
		}(i)
	}
}
