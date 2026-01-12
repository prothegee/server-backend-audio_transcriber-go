package pkg_audio_config

import (
	"encoding/json"
	"os"
)

type AudioConfig struct {
	Keywords struct {
		Forbidden struct {
			En []string `json:"en"`
		} `json:"forbidden"`
	} `json:"keywords"`
	Whisper struct {
		Model string `json:"model"`
	} `json:"whisper"`
	TimerAndTicker struct {
		SendingTicker int `json:"sending_ticker"` // in ms
	} `json:"timer_and_ticker"`
}

func AudioConfigLoad(fp string) (AudioConfig, error) {
	var cfg AudioConfig

	content, err := os.ReadFile(fp); if err != nil {
		return cfg, err
	}

	err = json.Unmarshal(content, &cfg); if err != nil {
		return cfg, err
	}

	return cfg, nil
}
