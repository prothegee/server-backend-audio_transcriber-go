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

    pkg "showcase-backend-audio_transcriber-go/pkg"
    pkg_audio "showcase-backend-audio_transcriber-go/pkg/audio"
    pkg_grpc "showcase-backend-audio_transcriber-go/pkg/grpc"
    pb "showcase-backend-audio_transcriber-go/protobuf"

    "github.com/google/uuid"
    "github.com/gordonklaus/portaudio"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

var (
    sampleRate float64
    framesPerBuf int
    audioChannels int
    audioBufferChannelSize int
)

func main() {
    // generate session id (uuid v7)
    sessionID, err := uuid.NewV7()
    if err != nil {
        log.Fatalf("failed to generate uuid v7: %v", err)
    }
    log.Printf("session id: %s", sessionID.String())

    err = portaudio.Initialize()
    if err != nil {
        log.Fatalf("fail to initialize portaudio: %v", err)
    }
    defer portaudio.Terminate()

    grpcCfg, err := pkg_grpc.GrpcConfigLoad("../../config.grpc.json")
    if err != nil {
        log.Fatalf("fail to load grpc config: %v", err)
    }
    audioCfg, err := pkg_audio.AudioConfigLoad("../../config.audio.json")
    if err != nil {
        log.Fatalf("fail to load audio config: %v", err)
    }

    sampleRate = audioCfg.Processing.SampleRate
    framesPerBuf = audioCfg.Processing.FramesPerBuf
    audioChannels = audioCfg.Processing.AudioChannels
    audioBufferChannelSize = audioCfg.Processing.AudioBufChannelSize

    // // uncomment this if you want to profile the program
    // // start pprof on :6061
    // go func() {
    //      log.Println("client pprof on :6061")
    //      log.Println(http.ListenAndServe("localhost:6061", nil))
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
        fmt.Printf("warning: device sample rate %.0f ≠ %f\n", device.DefaultSampleRate, sampleRate)
    }

    audioChan := make(chan []int16, audioBufferChannelSize)

    paramsInput := portaudio.StreamParameters{
        Input: portaudio.StreamDeviceParameters{
            Device:   device,
            Channels: audioChannels,
            Latency:  device.DefaultLowInputLatency,
        },
        SampleRate:      float64(sampleRate),
        FramesPerBuffer: framesPerBuf,
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
                // always receive feedback from server
                response, err := stream.Recv()
                if err != nil {
                    if err.Error() == "EOF" {
                        fmt.Println("server closed connection")
                        cancel()
                        return
                    }
                    log.Printf("note receiving feedback: %v", err)
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
        ticker := time.NewTicker(time.Duration(audioCfg.Processing.SendingTicker) * time.Millisecond)
        defer ticker.Stop()

        // buffer to accumulate audio > 1 second
        // 16 bit = 2 bytes per sample
        bytesPerSecond := int(sampleRate * 2) 
        var sendBuffer []byte

        for {
            select {
            case <-ctx.Done(): {
                return
            }
            case <-ticker.C: {
                // drain channel into local buffer
                drainLoop:
                for {
                    select {
                    case samples := <-audioChan:
                        bytes := pkg.Int16SliceToBytes(samples)
                        sendBuffer = append(sendBuffer, bytes...)
                    default:
                        break drainLoop
                    }
                }
                
                // only send if buffer is larger than 1 second (16000 * 2 bytes)
                if len(sendBuffer) >= bytesPerSecond {
                    if err := stream.Send(&pb.AudioChunk{
                        Data: sendBuffer,
                        SessionId: sessionID.String(),
                    }); err != nil {
                        log.Printf("send error: %v", err)
                        cancel()
                        return
                    }
                    // clear buffer after sending
                    sendBuffer = nil
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
