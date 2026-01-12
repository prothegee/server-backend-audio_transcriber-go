package pkg_grpc_config

import (
	"encoding/json"
	"os"
)

type GrpcConfig struct {
	Listener struct {
		Address string `json:"address"`
		Port int `json:"port"`
	} `json:"listener"`
	TimerAndTicker struct {
		AudioProcessing int `json:"audio_processing"` // in ms
	} `json:"timer_and_ticker"`
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
