package openai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type VisionRequest struct {
	Model      string
	Prompt     string
	MimeType   string
	ImageBytes []byte
}

func (c *Client) AnalyzeImage(ctx context.Context, req VisionRequest) (string, error) {
	if req.Model == "" {
		req.Model = "gpt-4.1-mini"
	}
	if req.Prompt == "" {
		req.Prompt = "请识别这张图片的主要内容，并用中文给出简洁结论。"
	}

	imageData := "data:" + req.MimeType + ";base64," + base64.StdEncoding.EncodeToString(req.ImageBytes)
	payload := map[string]interface{}{
		"model": req.Model,
		"messages": []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": req.Prompt},
					{"type": "image_url", "image_url": map[string]string{"url": imageData}},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("vision api error: %s", string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if len(result.Choices) == 0 {
		return "", fmt.Errorf("vision api returned empty choices")
	}
	return result.Choices[0].Message.Content, nil
}
