// cmd/audio_client/main.go
package main

import (
	"context"
	"fmt"
	"log"
	// "net/http"
	// _ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	pkg_audio_config "showcase-backend-audio_transcriber-go/pkg/audio"
	pkg_grpc_config "showcase-backend-audio_transcriber-go/pkg/grpc"
	pb "showcase-backend-audio_transcriber-go/protobuf"

	"github.com/gordonklaus/portaudio"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	sampleRate = 16000
	framesPerBuf = 512
	channels = 1
	audioBufferChannelSize = 1024
)

func int16SliceToBytes(data []int16) []byte {
	bytes := make([]byte, len(data)*2)
	for i, v := range data {
		bytes[i*2] = byte(v)
		bytes[i*2+1] = byte(v >> 8)
	}
	return bytes
}

func main() {
	err := portaudio.Initialize()
	if err != nil {
		log.Fatalf("fail to initialize portaudio: %v", err)
	}
	defer portaudio.Terminate()

	grpcCfg, err := pkg_grpc_config.GrpcConfigLoad("../../config.grpc.json")
	if err != nil {
		log.Fatalf("fail to load grpc config: %v", err)
	}
	audioCfg, err := pkg_audio_config.AudioConfigLoad("../../config.audio.json")
	if err != nil {
		log.Fatalf("fail to load audio config: %v", err)
	}

	// // uncomment this if you want to profile the program
	// // start pprof on :6061
	// go func() {
	// 	log.Println("client pprof on :6061")
	// 	log.Println(http.ListenAndServe("localhost:6061", nil))
	// }()

	grpcAddress := fmt.Sprintf("%s:%d", grpcCfg.Listener.Address, grpcCfg.Listener.Port)
	conn, err := grpc.NewClient(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("connection fail to %s: %v", grpcAddress, err)
	}
	defer conn.Close()

	client := pb.NewSpeechServiceClient(conn)

	// use cancellable context for clean shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.TranscribeStream(ctx)
	if err != nil {
		log.Fatalf("fail to create transcribe stream: %v", err)
	}

	// list available input devices
	devices, err := portaudio.Devices()
	if err != nil {
		log.Fatalf("failed to check devices: %v", err)
	}
	fmt.Printf("available input devices:\n")
	for i, dvc := range devices {
		if dvc.MaxInputChannels > 0 {
			fmt.Printf("#%d: %s\n", i, dvc.Name)
		}
	}

	device, err := portaudio.DefaultInputDevice()
	if err != nil {
		log.Fatalf("no default input device")
	}
	fmt.Printf("using: %s\n", device.Name)

	if device.DefaultSampleRate != sampleRate {
		fmt.Printf("warning: device sample rate %.0f ≠ %d\n", device.DefaultSampleRate, sampleRate)
	}

	audioChan := make(chan []int16, audioBufferChannelSize)

	paramsInput := portaudio.StreamParameters{
		Input: portaudio.StreamDeviceParameters{
			Device:   device,
			Channels: channels,
			Latency:  device.DefaultLowInputLatency,
		},
		SampleRate:       float64(sampleRate),
		FramesPerBuffer:  framesPerBuf,
	}

	streamCb, err := portaudio.OpenStream(paramsInput, func(in []int16) {
		if len(in) == 0 {
			return
		}
		buf := make([]int16, len(in))
		copy(buf, in)

		select {
		case audioChan <- buf: {
			// audio chunk accepted
		}
		default: {
			// buffer full → drop to avoid blocking real-time callback
			log.Print("audio buffer full, dropping chunk")
		}
		}
	})
	if err != nil {
		log.Fatalf("fail to open audio stream: %v", err)
	}
	defer streamCb.Close()

	err = streamCb.Start()
	if err != nil {
		log.Fatalf("fail to start audio stream: %v", err)
	}
	defer streamCb.Stop()

	// handle interrupt signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// feedback receiver goroutine
	go func() {
		for {
			select {
			case <-ctx.Done(): {
				return
			}
			default: {
				response, err := stream.Recv()
				if err != nil {
					if err.Error() == "EOF" {
						fmt.Println("server closed connection")
						cancel()
						return
					}
					log.Printf("error receiving feedback: %v", err)
					cancel()
					return
				}

				if response.Warning {
					fmt.Printf("\n\033[31m[warning] %s\033[0m\n", response.Text)
					if len(response.DetectedKeywords) > 0 {
						fmt.Printf("\033[31mkeywords: %v\033[0m\n", response.DetectedKeywords)
					}
				} else {
					fmt.Printf("\n\033[32m[pass] %s\033[0m\n", response.Text)
				}
			}
			}
		}
	}()

	// audio sender goroutine
	go func() {
		ticker := time.NewTicker(time.Duration(audioCfg.TimerAndTicker.SendingTicker) * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done(): {
				return
			}
			case <-ticker.C: {
				select {
				case samples := <-audioChan: {
					bytes := int16SliceToBytes(samples)
					if err := stream.Send(&pb.AudioChunk{Data: bytes}); err != nil {
						log.Printf("send error: %v", err)
						cancel()
						return
					}
				}
				default: {
					// no audio ready this tick; skip
				}
				}
			}
			}
		}
	}()

	fmt.Println("\nstart talking... press ctrl+c to stop")
	fmt.Println("--------------------------------------------------")

	<-sigChan
	fmt.Println("\nstopping audio client...")

	cancel()
	stream.CloseSend()

	time.Sleep(500 * time.Millisecond)
	fmt.Println("session finished")
}
