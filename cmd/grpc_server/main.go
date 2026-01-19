// cmd/grpc_server/main.go
package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	// "net/http"
	// _ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	pkg_audio_config "showcase-backend-audio_transcriber-go/pkg/audio"
	pkg_grpc_config "showcase-backend-audio_transcriber-go/pkg/grpc"
	pb "showcase-backend-audio_transcriber-go/protobuf"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"google.golang.org/grpc"
)

var (
	forbiddenEnKeywords       []string
	audioProcessingMs         int
	transcribeStreamChunkSize int
)

// transcribeRequest represents a request to transcribe audio
type transcribeRequest struct {
	Audio     []byte
	Resp      chan<- *transcribeResult
	Ctx       context.Context
	SessionID string
}

// transcribeResult is the result of transcription
type transcribeResult struct {
	Text     string
	Warning  bool
	Keywords []string
	Err      error
}

// whisperWorkerPool initializes a pool of workers to process requests concurrently
// we pass a *sync.Mutex to ensure only one inference runs at a time, prevent external lib SIGSEGV
func whisperWorkerPool(model whisper.Model, reqChan <-chan *transcribeRequest, numWorkers int, inferenceMu *sync.Mutex) {
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
				audioFloats, err := bytesToFloat32(req.Audio)
				if err != nil {
					select {
					case req.Resp <- &transcribeResult{Err: fmt.Errorf("audio conversion: %w", err)}:
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
					case req.Resp <- &transcribeResult{Err: fmt.Errorf("whisper process: %w", err)}:
					default:
						log.Printf("[worker #%d] resp chan full for session %s", workerID, req.SessionID)
					}
					continue
				}

				text := strings.TrimSpace(result.String())
				
				if text == "" || text == "BLANK_AUDIO" || len(text) < 2 {
					select {
					case req.Resp <- &transcribeResult{}:
					default:
					}
					continue
				}

				hasKeywords, found := containsKeywords(text, forbiddenEnKeywords)
				res := &transcribeResult{
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

func bytesToFloat32(data []byte) ([]float32, error) {
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

func containsKeywords(text string, keywords []string) (bool, []string) {
	found := []string{}
	lowerText := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lowerText, strings.ToLower(kw)) {
			found = append(found, kw)
		}
	}
	return len(found) > 0, found
}

type server struct {
	pb.UnimplementedSpeechServiceServer
	reqChan chan *transcribeRequest
}

func (s *server) TranscribeStream(stream pb.SpeechService_TranscribeStreamServer) error {
	var buffer bytes.Buffer
	currentSessionID := "unknown-session"

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	feedbackChan := make(chan *pb.Transcript, 50)

	log.Printf("new client connected")

	// feedback sender
	go func() {
		defer close(feedbackChan)
		for {
			select {
			case <-ctx.Done():
				return
			case fb, ok := <-feedbackChan:
				if !ok {
					return
				}
				if err := stream.Send(fb); err != nil {
					log.Printf("[%s] send feedback error: %v", currentSessionID, err)
					return
				}
			}
		}
	}()

	// audio processor trigger
	processTicker := time.NewTicker(time.Duration(audioProcessingMs) * time.Millisecond)
	defer processTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-processTicker.C:
				if buffer.Len() > 0 {
					audioData := buffer.Bytes()

					if len(audioData) == 0 {
						continue
					}

					dataToSend := make([]byte, len(audioData))
					copy(dataToSend, audioData)
					buffer.Reset()

					respChan := make(chan *transcribeResult, 1)
					req := &transcribeRequest{
						Audio:     dataToSend,
						Resp:      respChan,
						Ctx:       ctx,
						SessionID: currentSessionID,
					}

					select {
					case s.reqChan <- req:
						// request queued
					case <-ctx.Done():
						return
					default:
						log.Printf("[%s] dropping chunk: workers busy", currentSessionID)
						continue
					}

					// listen for response in background
					go func(sessionID string) {
						var res *transcribeResult
						select {
						case res = <-respChan:
							// got result
						case <-ctx.Done():
							return
						case <-time.After(15 * time.Second):
							log.Printf("[%s] transcription timeout", sessionID)
							return
						}

						if res.Err != nil {
							log.Printf("[%s] transcription error: %v", sessionID, res.Err)
							return
						}

						var fb *pb.Transcript
						if res.Warning {
							fb = &pb.Transcript{
								Text:             fmt.Sprintf("detected forbidden keyword: %v - '%s'", res.Keywords, res.Text),
								Warning:          true,
								DetectedKeywords: res.Keywords,
							}
							log.Printf("[%s] forbidden keywords detected: %v", sessionID, res.Keywords)
						} else if res.Text != "" {
							fb = &pb.Transcript{
								Text:    fmt.Sprintf("ok: '%s'", res.Text),
								Warning: false,
							}
							log.Printf("[%s] processed: '%s'", sessionID, res.Text)
						}

						if fb != nil {
							select {
							case feedbackChan <- fb:
							case <-ctx.Done():
								return
							default:
								log.Printf("[%s] timeout sending feedback", sessionID)
							}
						}
					}(currentSessionID)
				}
			}
		}
	}()

	// receive audio chunks from client
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			chunk, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					log.Printf("[%s] client disconnected", currentSessionID)
					return nil
				}
				log.Printf("[%s] stream recv error: %v", currentSessionID, err)
				return err
			}

			// ignore empty data
			if len(chunk.Data) == 0 {
				continue
			}

			// update session id if present
			if chunk.SessionId != "" && currentSessionID == "unknown-session" {
				currentSessionID = chunk.SessionId
				log.Printf("session identified: %s", currentSessionID)
			}

			buffer.Write(chunk.Data)

			if buffer.Len() > transcribeStreamChunkSize*10 {
				log.Printf("[%s] buffer overflow, resetting", currentSessionID)
				buffer.Reset()
			}
		}
	}
}

func main() {
	grpcCfg, err := pkg_grpc_config.GrpcConfigLoad("../../config.grpc.json")
	if err != nil {
		log.Fatalf("failed to load grpc config: %v", err)
	}
	audioCfg, err := pkg_audio_config.AudioConfigLoad("../../config.audio.json")
	if err != nil {
		log.Fatalf("failed to load audio config: %v", err)
	}

	forbiddenEnKeywords = audioCfg.Keywords.Forbidden.En
	audioProcessingMs = grpcCfg.Processing.AudioProcessing
	transcribeStreamChunkSize = grpcCfg.Processing.TranscribeStreamChunkSize

	// load whisper model once
	model, err := whisper.New(audioCfg.Whisper.Model)
	if err != nil {
		log.Fatalf("failed to load whisper model: %v", err)
	}
	defer model.Close()
	log.Printf("whisper model loaded: %s", audioCfg.Whisper.Model)

	// note: 
	// - this one is to protect cgo inference calls
	// - it makes compute serial, but concurrent preparation
	var inferenceMu sync.Mutex

	numWorkers := runtime.NumCPU()
	if numWorkers < 2 {
		numWorkers = 2
		log.Fatal("total detected workers is less than 2")
	}

	// buffer channel larger to handle burst traffic
	reqChan := make(chan *transcribeRequest, 100)
	
	log.Printf("starting %d whisper workers", numWorkers)
	whisperWorkerPool(model, reqChan, numWorkers, &inferenceMu)

	lis, err := net.Listen("tcp", fmt.Sprintf("%s:%d", grpcCfg.Listener.Address, grpcCfg.Listener.Port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterSpeechServiceServer(grpcServer, &server{reqChan: reqChan})

	// graceful shutdown on signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Print("shutting down grpc server...")
		grpcServer.GracefulStop()
		close(reqChan)
	}()

	log.Printf("server running on %s:%d", grpcCfg.Listener.Address, grpcCfg.Listener.Port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
