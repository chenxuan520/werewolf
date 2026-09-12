package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"werewolf/backend/internal/config"
)

// Transcribe 把一段 wav 音频（base64，不含 data: 前缀）发给 OpenAI 兼容的
// chat/completions + input_audio 接口，返回识别出的文本。
// 实现参考 /Users/bytedance/self/voice2text 的 mimo provider。
func (c *Client) Transcribe(ctx context.Context, asr config.ASRConfig, audioBase64 string) (string, error) {
	endpoint := transcribeEndpoint(asr.Endpoint)
	language := strings.TrimSpace(asr.Language)
	if language == "" {
		language = "auto"
	}
	payload := map[string]any{
		"model": asr.Model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "input_audio",
						"input_audio": map[string]any{
							"data": "data:audio/wav;base64," + audioBase64,
						},
					},
				},
			},
		},
		"asr_options": map[string]any{
			"language": language,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	// mimo 用 api-key 头；同时带 Authorization 兼容其它 OpenAI 兼容端点。
	req.Header.Set("api-key", asr.Token)
	req.Header.Set("Authorization", "Bearer "+asr.Token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(rawBody))
		if len(message) > 300 {
			message = message[:300] + "..."
		}
		return "", fmt.Errorf("asr http %d: %s", resp.StatusCode, message)
	}
	var completion struct {
		Choices []struct {
			Message completionMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawBody, &completion); err != nil {
		return "", err
	}
	if len(completion.Choices) == 0 {
		return "", fmt.Errorf("empty asr response")
	}
	text := strings.TrimSpace(completion.Choices[0].Message.Content.String())
	if text == "" {
		return "", fmt.Errorf("asr returned empty transcript")
	}
	return text, nil
}

func transcribeEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}
