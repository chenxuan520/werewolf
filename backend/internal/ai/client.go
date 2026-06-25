package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"werewolf/backend/internal/config"
)

type Client struct {
	httpClient *http.Client
}

type TurnInput struct {
	Objective      string       `json:"objective"`
	Template       string       `json:"template"`
	Day            int          `json:"day"`
	Phase          string       `json:"phase"`
	YourSeat       int          `json:"yourSeat"`
	YourName       string       `json:"yourName"`
	YourRole       string       `json:"yourRole"`
	Persona        string       `json:"persona,omitempty"`
	AlivePlayers   []TurnPlayer `json:"alivePlayers"`
	PublicLog      []string     `json:"publicLog,omitempty"`
	PrivateNotes   []string     `json:"privateNotes,omitempty"`
	ValidTargets   []TurnTarget `json:"validTargets,omitempty"`
	NightVictim    *TurnTarget  `json:"nightVictim,omitempty"`
	CanUseHeal     bool         `json:"canUseHeal,omitempty"`
	CanUsePoison   bool         `json:"canUsePoison,omitempty"`
	AllowSpeech    bool         `json:"allowSpeech,omitempty"`
	AllowTarget    bool         `json:"allowTarget,omitempty"`
	AllowWitchMode bool         `json:"allowWitchMode,omitempty"`
}

type TurnPlayer struct {
	Seat         int    `json:"seat"`
	Name         string `json:"name"`
	Alive        bool   `json:"alive"`
	RevealedRole string `json:"revealedRole,omitempty"`
}

type TurnTarget struct {
	Seat int    `json:"seat"`
	Name string `json:"name"`
}

type Decision struct {
	Speech           string `json:"speech,omitempty"`
	TargetSeat       *int   `json:"target_seat,omitempty"`
	UseHeal          bool   `json:"use_heal,omitempty"`
	PoisonTargetSeat *int   `json:"poison_target_seat,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type ProbeResult struct {
	OK              bool   `json:"ok"`
	LatencyMs       int64  `json:"latencyMs"`
	Model           string `json:"model,omitempty"`
	ResponseSnippet string `json:"responseSnippet,omitempty"`
	Error           string `json:"error,omitempty"`
}

func NewClient() *Client {
	return &Client{httpClient: &http.Client{Timeout: 45 * time.Second}}
}

func (c *Client) Decide(ctx context.Context, preset config.Preset, input TurnInput) (Decision, error) {
	var lastErr error
	lastHint := ""
	for attempt := 1; attempt <= 3; attempt++ {
		decision, err := c.decideOnce(ctx, preset, input, attempt, lastHint)
		if err == nil {
			return decision, nil
		}
		lastErr = err
		lastHint = retryHintFromError(err.Error())
		if ctx.Err() != nil || attempt == 3 {
			break
		}
		time.Sleep(time.Duration(attempt) * 250 * time.Millisecond)
	}
	return Decision{}, lastErr
}

func (c *Client) decideOnce(ctx context.Context, preset config.Preset, input TurnInput, attempt int, lastHint string) (Decision, error) {
	body, err := json.Marshal(buildRequestPayload(preset, input, attempt, lastHint))
	if err != nil {
		return Decision{}, err
	}
	endpoint := strings.TrimRight(preset.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return Decision{}, err
	}
	applyRequestHeaders(req, preset)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Decision{}, err
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return Decision{}, err
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(rawBody))
		if len(message) > 300 {
			message = message[:300] + "..."
		}
		return Decision{}, fmt.Errorf("http %d: %s", resp.StatusCode, message)
	}
	var completion struct {
		Choices []struct {
			Message completionMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawBody, &completion); err != nil {
		return Decision{}, err
	}
	if len(completion.Choices) == 0 {
		return Decision{}, fmt.Errorf("empty ai response")
	}
	return parseTurnResponse(completion.Choices[0].Message.Content.String(), completion.Choices[0].Message.ToolCalls)
}

func (c *Client) Probe(ctx context.Context, preset config.Preset) ProbeResult {
	payload := map[string]any{
		"model":      preset.Model,
		"max_tokens": 16,
		"messages": []map[string]string{
			{"role": "user", "content": "Reply with exactly pong."},
		},
	}
	mergeExtraBody(payload, preset.ExtraBody)
	body, err := json.Marshal(payload)
	if err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}
	endpoint := strings.TrimRight(preset.Endpoint, "/") + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return ProbeResult{OK: false, Error: err.Error()}
	}
	applyRequestHeaders(req, preset)
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return ProbeResult{OK: false, LatencyMs: latency, Error: err.Error()}
	}
	defer resp.Body.Close()
	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ProbeResult{OK: false, LatencyMs: latency, Error: err.Error()}
	}
	if resp.StatusCode >= 400 {
		message := strings.TrimSpace(string(rawBody))
		if len(message) > 200 {
			message = message[:200] + "..."
		}
		return ProbeResult{OK: false, LatencyMs: latency, Error: fmt.Sprintf("http %d: %s", resp.StatusCode, message)}
	}
	var completion struct {
		Model   string `json:"model"`
		Choices []struct {
			Message completionMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(rawBody, &completion); err != nil {
		return ProbeResult{OK: false, LatencyMs: latency, Error: err.Error()}
	}
	snippet := ""
	if len(completion.Choices) > 0 {
		snippet = strings.TrimSpace(completion.Choices[0].Message.Content.String())
	}
	if len(snippet) > 80 {
		snippet = snippet[:80] + "..."
	}
	model := completion.Model
	if model == "" {
		model = preset.Model
	}
	return ProbeResult{OK: true, LatencyMs: latency, Model: model, ResponseSnippet: snippet}
}

func buildRequestPayload(preset config.Preset, input TurnInput, attempt int, lastHint string) map[string]any {
	mode := preset.StructuredOutputMode()
	messages := []map[string]string{
		{"role": "system", "content": systemInstruction(preset.SystemPrompt, mode)},
		{"role": "user", "content": mustJSON(input)},
	}
	if attempt > 1 {
		messages = append(messages, map[string]string{
			"role":    "user",
			"content": retryReminder(attempt, lastHint),
		})
	}
	payload := map[string]any{
		"model":       preset.Model,
		"temperature": 0.4,
		"max_tokens":  maxTokensFor(preset),
		"messages":    messages,
	}
	switch mode {
	case config.StructuredOutputToolCall:
		payload["tools"] = turnTools()
	case config.StructuredOutputJSONObject:
		payload["response_format"] = map[string]string{"type": "json_object"}
	case config.StructuredOutputNone:
		// rely on prompt + parser fallback
	}
	mergeExtraBody(payload, preset.ExtraBody)
	return payload
}

func retryHintFromError(rawErr string) string {
	hint := strings.TrimSpace(rawErr)
	if hint == "" {
		return ""
	}
	if len(hint) > 160 {
		hint = hint[:160] + "..."
	}
	return hint
}

func retryReminder(attempt int, lastHint string) string {
	if lastHint == "" {
		return fmt.Sprintf("第%d次重试：请只输出一个完整 JSON，对应当前回合唯一动作。", attempt)
	}
	return fmt.Sprintf("第%d次重试，上次失败原因：%s。请只输出完整合法 JSON。", attempt, lastHint)
}

func systemInstruction(prompt string, mode string) string {
	var b strings.Builder
	if trimmed := strings.TrimSpace(prompt); trimmed != "" {
		b.WriteString(trimmed)
		b.WriteString("\n\n")
	}
	b.WriteString("你在参加中文狼人杀局。你只需要给出当前回合的唯一合法动作。\n")
	b.WriteString("【基本要求】只根据给到的信息行动，不要编造额外角色、查验结果或夜晚信息。座位号必须从 validTargets 中选择。\n")
	b.WriteString("【发言轮】如果 allowSpeech=true，speech 写 1~2 句自然中文，控制在 60 字内，像真人发言，不要写分析提纲。\n")
	b.WriteString("【选目标轮】如果 allowTarget=true，填写 target_seat。\n")
	b.WriteString("【女巫轮】如果 allowWitchMode=true，可用 use_heal 决定是否救人；若要下毒，poison_target_seat 必须来自 validTargets。\n")
	b.WriteString("【风格】如果 persona 不为空，可参考它；但仍要优先合法、可落地。\n")
	switch mode {
	case config.StructuredOutputToolCall:
		b.WriteString("【输出】调用 submit_turn 工具，禁止额外文本。")
	case config.StructuredOutputJSONObject:
		b.WriteString("【输出】只返回一个 JSON 对象，禁止 markdown 和额外解释。")
	default:
		b.WriteString("【输出】只返回一个 JSON 对象，禁止额外解释。")
	}
	b.WriteString(" reason 可选，最多 20 字。")
	return b.String()
}

func turnTools() []map[string]any {
	return []map[string]any{{
		"type": "function",
		"function": map[string]any{
			"name":        "submit_turn",
			"description": "提交当前狼人杀回合动作",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"speech":             map[string]any{"type": "string"},
					"target_seat":        map[string]any{"type": "integer"},
					"use_heal":           map[string]any{"type": "boolean"},
					"poison_target_seat": map[string]any{"type": "integer"},
					"reason":             map[string]any{"type": "string"},
				},
			},
		},
	}}
}

func applyRequestHeaders(req *http.Request, preset config.Preset) {
	req.Header.Set("Authorization", "Bearer "+preset.Token)
	req.Header.Set("Content-Type", "application/json")
	for key, value := range preset.ExtraHeaders {
		name := strings.TrimSpace(key)
		trimmed := strings.TrimSpace(value)
		if name == "" || trimmed == "" {
			continue
		}
		switch strings.ToLower(name) {
		case "authorization", "content-type":
			continue
		default:
			req.Header.Set(name, trimmed)
		}
	}
}

func mergeExtraBody(payload map[string]any, extra map[string]any) {
	for k, v := range extra {
		if v == nil {
			delete(payload, k)
			continue
		}
		payload[k] = v
	}
}

func maxTokensFor(preset config.Preset) int {
	if preset.MaxTokens > 0 {
		return preset.MaxTokens
	}
	return 220
}

func mustJSON(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}
