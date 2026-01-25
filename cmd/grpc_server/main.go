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
	"sync"
	"syscall"
	"time"

	pkg_audio "showcase-backend-audio_transcriber-go/pkg/audio"
	pkg_grpc "showcase-backend-audio_transcriber-go/pkg/grpc"
	pkg_whisper "showcase-backend-audio_transcriber-go/pkg/whisper"
	pb "showcase-backend-audio_transcriber-go/protobuf"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"google.golang.org/grpc"
)

var (
	forbiddenEnKeywords       []string
	audioProcessingMs         int
	transcribeStreamChunkSize int
)

type server struct {
	pb.UnimplementedSpeechServiceServer
	reqChan chan *pkg_audio.TranscribeRequest
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

					respChan := make(chan *pkg_audio.TranscribeResult, 1)
					req := &pkg_audio.TranscribeRequest{
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
						var res *pkg_audio.TranscribeResult
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
	grpcCfg, err := pkg_grpc.GrpcConfigLoad("../../config.grpc.json")
	if err != nil {
		log.Fatalf("failed to load grpc config: %v", err)
	}
	audioCfg, err := pkg_audio.AudioConfigLoad("../../config.audio.json")
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
	reqChan := make(chan *pkg_audio.TranscribeRequest, 100)
	
	log.Printf("starting %d whisper workers", numWorkers)
	pkg_whisper.WhisperWorkerPool(model, reqChan, numWorkers, &inferenceMu, forbiddenEnKeywords)

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
