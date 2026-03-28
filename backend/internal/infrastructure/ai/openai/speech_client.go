package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

type TTSRequest struct {
	Model string
	Voice string
	Text  string
	Format string
}

type TTSResponse struct {
	AudioBase64 string
	MIMEType    string
}

type ASRRequest struct {
	Model      string
	Language   string
	Prompt     string
	FileName   string
	AudioBytes []byte
}

func (c *Client) TextToSpeech(ctx context.Context, req TTSRequest) (*TTSResponse, error) {
	if req.Model == "" {
		req.Model = "gpt-4o-mini-tts"
	}
	if req.Voice == "" {
		req.Voice = "alloy"
	}
	if req.Format == "" {
		req.Format = "mp3"
	}

	payload := map[string]interface{}{
		"model": req.Model,
		"voice": req.Voice,
		"input": req.Text,
		"format": req.Format,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/audio/speech", bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("speech api error: %s", string(payload))
	}

	audio, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	mimeType := resp.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "audio/mpeg"
	}

	return &TTSResponse{AudioBase64: base64.StdEncoding.EncodeToString(audio), MIMEType: mimeType}, nil
}

func (c *Client) SpeechToText(ctx context.Context, req ASRRequest) (string, error) {
	if req.Model == "" {
		req.Model = "whisper-1"
	}
	if req.FileName == "" {
		req.FileName = "audio.wav"
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", req.Model)
	if req.Language != "" {
		_ = writer.WriteField("language", req.Language)
	}
	if req.Prompt != "" {
		_ = writer.WriteField("prompt", req.Prompt)
	}
	fileWriter, err := writer.CreateFormFile("file", req.FileName)
	if err != nil {
		return "", err
	}
	if _, err := fileWriter.Write(req.AudioBytes); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		payload, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("asr api error: %s", string(payload))
	}

	var result struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	return result.Text, nil
}
