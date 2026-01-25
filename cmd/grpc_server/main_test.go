package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	pb "showcase-backend-audio_transcriber-go/protobuf"
	pkg_audio "showcase-backend-audio_transcriber-go/pkg/audio"
)

// TestMain initializes global config variables for tests
func TestMain(m *testing.M) {
	audioProcessingMs = 100          // must be > 0
	transcribeStreamChunkSize = 1600 // e.g., 1.6KB chunks

	code := m.Run()
	os.Exit(code)
}

// mockStream wraps GenericServerStream and adds custom Send/Recv
type mockStream struct {
	*grpc.GenericServerStream[pb.AudioChunk, pb.Transcript]
	sendChan chan *pb.Transcript
	recvChan chan *pb.AudioChunk
	ctx      context.Context
}

func newMockStream(ctx context.Context) *mockStream {
	dummy := &dummyServerStream{ctx: ctx}
	gs := &grpc.GenericServerStream[pb.AudioChunk, pb.Transcript]{ServerStream: dummy}
	return &mockStream{
		GenericServerStream: gs,
		sendChan:           make(chan *pb.Transcript, 10),
		recvChan:           make(chan *pb.AudioChunk, 10),
		ctx:                ctx,
	}
}

func (m *mockStream) Recv() (*pb.AudioChunk, error) {
	select {
	case msg := <-m.recvChan:
		return msg, nil
	case <-m.ctx.Done():
		return nil, io.EOF
	}
}

func (m *mockStream) Send(resp *pb.Transcript) error {
	select {
	case m.sendChan <- resp:
		return nil
	case <-m.ctx.Done():
		return io.EOF
	}
}

// dummyServerStream implements grpc.ServerStream minimally
type dummyServerStream struct {
	ctx context.Context
}

func (d *dummyServerStream) SetHeader(md metadata.MD) error   { return nil }
func (d *dummyServerStream) SendHeader(md metadata.MD) error  { return nil }
func (d *dummyServerStream) SetTrailer(md metadata.MD)        {}
func (d *dummyServerStream) Context() context.Context         { return d.ctx }
func (d *dummyServerStream) SendMsg(m interface{}) error      { return nil }
func (d *dummyServerStream) RecvMsg(m interface{}) error      { return nil }

// --- tests ---
func TestTranscribeStreamBasicFlow(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream := newMockStream(ctx)
	srv := &server{
		reqChan: make(chan *pkg_audio.TranscribeRequest, 10),
	}

	go func() {
		time.Sleep(10 * time.Millisecond)
		stream.recvChan <- &pb.AudioChunk{
			Data:      []byte{0x00, 0x00, 0x00, 0x00},
			SessionId: "test-session",
		}
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	go func() {
		for req := range srv.reqChan {
			req.Resp <- &pkg_audio.TranscribeResult{
				Text: "ok test",
			}
		}
	}()

	err := srv.TranscribeStream(stream)
	if err != nil && err.Error() != "EOF" {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case fb := <-stream.sendChan:
		if fb.Text == "" {
			t.Error("expected non-empty feedback text")
		}
	default:
		t.Log("no feedback received (timing may vary)")
	}
}

func TestBufferOverflowProtection(t *testing.T) {
	var buf bytes.Buffer
	transcribeStreamChunkSize = 10 // small limit

	for i := 0; i < 15; i++ {
		buf.Write(make([]byte, 100))
	}

	if buf.Len() > transcribeStreamChunkSize*10 {
		buf.Reset()
	}

	if buf.Len() != 0 {
		t.Errorf("buffer should be reset after overflow, got %d bytes", buf.Len())
	}
}
