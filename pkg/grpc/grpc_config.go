package pkg_grpc

import (
	"encoding/json"
	"os"
)

type GrpcConfig struct {
	Listener struct {
		Address string `json:"address"`
		Port int `json:"port"`
	} `json:"listener"`
	Processing struct {
		AudioProcessing int `json:"audio_processing"` // in ms
		TranscribeStreamChunkSize int `json:"transcribe_stream_chunk_size"`
	} `json:"processing"`
}

func GrpcConfigLoad(fp string) (GrpcConfig, error) {
	var cfg GrpcConfig

	content, err := os.ReadFile(fp); if err != nil {
		return cfg, err
	}

	err = json.Unmarshal(content, &cfg); if err != nil {
		return cfg, err
	}

	return cfg, nil
}
