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
	"strings"
	"syscall"
	"time"

	pkg_audio_config "showcase-backend-audio_transcriber-go/pkg/audio"
	pkg_grpc_config "showcase-backend-audio_transcriber-go/pkg/grpc"
	pb "showcase-backend-audio_transcriber-go/protobuf"

	whisper "github.com/ggerganov/whisper.cpp/bindings/go/pkg/whisper"
	"google.golang.org/grpc"
)

var (
	forbiddenEnKeywords []string
	audioProcessingMs int
)

// transcribeRequest represents a request to transcribe audio
type transcribeRequest struct {
	Audio []byte
	Resp chan<- *transcribeResult
	Ctx context.Context
}

// transcribeResult is the result of transcription
type transcribeResult struct {
	Text string
	Warning bool
	Keywords []string
	Err error
}

// whisperWorker runs in a single goroutine and processes transcription requests
func whisperWorker(modelPath string, reqChan <-chan *transcribeRequest) {
	model, err := whisper.New(modelPath)
	if err != nil {
		log.Fatalf("failed to load whisper model: %v", err)
	}
	defer model.Close()

	ctx, err := model.NewContext()
	if err != nil {
		log.Fatalf("failed to create whisper context: %v", err)
	}

	ctx.SetLanguage("auto")
	ctx.SetTranslate(false)

	log.Printf("whisper model loaded: %s", modelPath)

	for req := range reqChan {
		select {
		case <-req.Ctx.Done(): {
			req.Resp <- &transcribeResult{Err: req.Ctx.Err()}
			continue
		}
		default: {
			// proceed with transcription
		}
		}

		audioFloats, err := bytesToFloat32(req.Audio)
		if err != nil {
			req.Resp <- &transcribeResult{Err: fmt.Errorf("audio conversion: %w", err)}
			continue
		}

		var result strings.Builder
		segmentCallback := func(segment whisper.Segment) {
			result.WriteString(segment.Text)
		}

		err = ctx.Process(audioFloats, nil, segmentCallback, nil)
		if err != nil {
			req.Resp <- &transcribeResult{Err: fmt.Errorf("whisper process: %w", err)}
			continue
		}

		text := strings.TrimSpace(result.String())
		if text == "" || text == "BLANK_AUDIO" {
			req.Resp <- &transcribeResult{}
			continue
		}

		hasKeywords, found := containsKeywords(text, forbiddenEnKeywords)
		req.Resp <- &transcribeResult{
			Text: text,
			Warning: hasKeywords,
			Keywords: found,
		}
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
	const chunkSize = 32000
	var buffer bytes.Buffer

	ctx, cancel := context.WithCancel(stream.Context())
	defer cancel()

	feedbackChan := make(chan *pb.Transcript, 50)

	log.Printf("real-time audio transcriber stream started")

	// feedback sender
	go func() {
		defer close(feedbackChan)
		for {
			select {
			case <-ctx.Done(): {
				return
			}
			case fb, ok := <-feedbackChan: {
				if !ok {
					return
				}
				if err := stream.Send(fb); err != nil {
					log.Printf("send feedback error: %v", err)
					return
				}
			}
			}
		}
	}()

	// audio processor
	processTicker := time.NewTicker(time.Duration(audioProcessingMs) * time.Millisecond)
	defer processTicker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done(): {
				return
			}
			case <-processTicker.C: {
				if buffer.Len() >= chunkSize {
					audioData := make([]byte, chunkSize)
					n, _ := buffer.Read(audioData)
					if n == 0 {
						continue
					}

					respChan := make(chan *transcribeResult, 1)
					req := &transcribeRequest{
						Audio: audioData,
						Resp: respChan,
						Ctx: ctx,
					}

					select {
					case s.reqChan <- req: {
						// request queued
					}
					case <-ctx.Done(): {
						return
					}
					case <-time.After(100 * time.Millisecond): {
						log.Print("dropping chunk: worker busy")
						continue
					}
					}

					go func() {
						var res *transcribeResult
						select {
						case res = <-respChan: {
							// got result
						}
						case <-ctx.Done(): {
							return
						}
						case <-time.After(10 * time.Second): {
							res = &transcribeResult{Err: fmt.Errorf("transcription timeout")}
						}
						}

						if res.Err != nil {
							log.Printf("transcription error: %v", res.Err)
							return
						}

						var fb *pb.Transcript
						if res.Warning {
							fb = &pb.Transcript{
								Text: fmt.Sprintf("detected forbidden keyword: %v - '%s'", res.Keywords, res.Text),
								Warning: true,
								DetectedKeywords: res.Keywords,
							}
							log.Printf("forbidden keywords detected: %v", res.Keywords)
						} else if res.Text != "" {
							fb = &pb.Transcript{
								Text: fmt.Sprintf("ok: '%s'", res.Text),
								Warning: false,
							}
						}

						if fb != nil {
							select {
							case feedbackChan <- fb: {
								// sent
							}
							case <-ctx.Done(): {
								return
							}
							case <-time.After(100 * time.Millisecond): {
								log.Print("timeout sending feedback")
							}
							}
						}
					}()
				}
			}
			}
		}
	}()

	// receive audio chunks from client
	for {
		select {
		case <-ctx.Done(): {
			return nil
		}
		default: {
			chunk, err := stream.Recv()
			if err != nil {
				if err.Error() == "EOF" {
					log.Print("client disconnected")
					return nil
				}
				return err
			}
			buffer.Write(chunk.Data)
			if buffer.Len() > chunkSize*2 {
				buffer.Next(buffer.Len() - chunkSize)
			}
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
	audioProcessingMs = grpcCfg.TimerAndTicker.AudioProcessing

	// // uncomment this if you want to profile the program
	// // start pprof http server on :6060
	// go func() {
	// 	log.Println("pprof server running on :6060")
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()

	// initialize whisper worker
	reqChan := make(chan *transcribeRequest, 10)
	go whisperWorker(audioCfg.Whisper.Model, reqChan)

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
