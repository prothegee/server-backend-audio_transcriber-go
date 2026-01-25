package pkg_audio

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
	Processing struct {
		SendingTicker int `json:"sending_ticker"` // in ms
		SampleRate float64 `json:"sample_rate"`
		FramesPerBuf int `json:"frames_per_buf"`
		AudioChannels int `json:"audio_channels"`
		AudioBufChannelSize int `json:"audio_buf_channel_size"`
	} `json:"processing"`
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
