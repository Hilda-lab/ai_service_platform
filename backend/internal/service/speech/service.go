package speech

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

type TTSFunc func(ctx context.Context, req TTSRequest) (*TTSResult, error)

type ASRFunc func(ctx context.Context, req ASRRequest) (string, error)

type TTSRequest struct {
	Model    string
	Voice    string
	Text     string
	Format   string
	Language string
}

type TTSResult struct {
	AudioBase64 string `json:"audioBase64"`
	MIMEType    string `json:"mimeType"`
}

type ASRRequest struct {
	Model      string
	Language   string
	Prompt     string
	FileName   string
	AudioBytes []byte
}

type Service struct {
	tts            TTSFunc
	asr            ASRFunc
	provider       string
	defaultTTSModel string
	defaultASRModel string
	defaultVoice   string
	defaultLang    string
	mock           bool
}

func NewService(tts TTSFunc, asr ASRFunc, provider, ttsModel, asrModel, voice, lang string, mock bool) *Service {
	if provider == "" {
		provider = "openai"
	}
	if ttsModel == "" {
		ttsModel = "gpt-4o-mini-tts"
	}
	if asrModel == "" {
		asrModel = "whisper-1"
	}
	if voice == "" {
		voice = "alloy"
	}
	if lang == "" {
		lang = "zh"
	}
	return &Service{tts: tts, asr: asr, provider: provider, defaultTTSModel: ttsModel, defaultASRModel: asrModel, defaultVoice: voice, defaultLang: lang, mock: mock}
}

func (s *Service) Synthesize(ctx context.Context, req TTSRequest) (*TTSResult, error) {
	text := strings.TrimSpace(req.Text)
	if text == "" {
		return nil, errors.New("text is required")
	}
	if req.Model == "" {
		req.Model = s.defaultTTSModel
	}
	if req.Voice == "" {
		req.Voice = s.defaultVoice
	}
	if req.Language == "" {
		req.Language = s.defaultLang
	}
	if req.Format == "" {
		req.Format = "mp3"
	}

	if s.mock {
		return &TTSResult{AudioBase64: base64.StdEncoding.EncodeToString([]byte("mock tts audio")), MIMEType: "audio/mpeg"}, nil
	}
	if s.tts == nil {
		return nil, errors.New("tts engine is not configured")
	}
	return s.tts(ctx, req)
}

func (s *Service) Transcribe(ctx context.Context, req ASRRequest) (string, error) {
	if len(req.AudioBytes) == 0 {
		return "", errors.New("audio file is required")
	}
	if req.Model == "" {
		req.Model = s.defaultASRModel
	}
	if req.Language == "" {
		req.Language = s.defaultLang
	}
	if s.mock {
		return fmt.Sprintf("[mock] 语音识别成功，音频大小 %d bytes", len(req.AudioBytes)), nil
	}
	if s.asr == nil {
		return "", errors.New("asr engine is not configured")
	}
	return s.asr(ctx, req)
}
