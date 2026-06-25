package ai

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

type ToolCall struct {
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type completionMessage struct {
	Content   completionContent `json:"content"`
	ToolCalls []ToolCall        `json:"tool_calls"`
}

type completionContent string

func (c completionContent) String() string {
	return string(c)
}

func (c *completionContent) UnmarshalJSON(data []byte) error {
	var plain string
	if err := json.Unmarshal(data, &plain); err == nil {
		*c = completionContent(plain)
		return nil
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(data, &blocks); err == nil {
		parts := make([]string, 0, len(blocks))
		for _, block := range blocks {
			if strings.TrimSpace(block.Text) == "" {
				continue
			}
			if block.Type != "" && block.Type != "text" {
				continue
			}
			parts = append(parts, block.Text)
		}
		*c = completionContent(strings.Join(parts, "\n"))
		return nil
	}
	return fmt.Errorf("unsupported completion content shape")
}

func parseTurnResponse(content string, toolCalls []ToolCall) (Decision, error) {
	for _, toolCall := range toolCalls {
		if strings.TrimSpace(toolCall.Function.Name) != "submit_turn" {
			continue
		}
		decision, err := parseDecisionArguments(toolCall.Function.Arguments)
		if err == nil {
			return normalizeDecision(decision), nil
		}
	}
	decision, err := parseDecision(content)
	if err != nil {
		return Decision{}, err
	}
	return normalizeDecision(decision), nil
}

func parseDecision(content string) (Decision, error) {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "```") {
		trimmed = strings.TrimPrefix(trimmed, "```json")
		trimmed = strings.TrimPrefix(trimmed, "```")
		trimmed = strings.TrimSuffix(trimmed, "```")
		trimmed = strings.TrimSpace(trimmed)
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		trimmed = trimmed[start : end+1]
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return Decision{}, fmt.Errorf("parse decision json: %w", err)
	}
	return decisionFromMap(raw)
}

func parseDecisionArguments(arguments string) (Decision, error) {
	trimmed := strings.TrimSpace(arguments)
	if trimmed == "" {
		return Decision{}, fmt.Errorf("tool arguments are empty")
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return Decision{}, err
	}
	return decisionFromMap(raw)
}

func decisionFromMap(raw map[string]any) (Decision, error) {
	decision := Decision{}
	if speech, ok := raw["speech"].(string); ok {
		decision.Speech = strings.TrimSpace(speech)
	}
	if reason, ok := raw["reason"].(string); ok {
		decision.Reason = strings.TrimSpace(reason)
	}
	if value, ok := parseOptionalInt(raw["target_seat"]); ok {
		decision.TargetSeat = &value
	}
	if value, ok := parseOptionalBool(raw["use_heal"]); ok {
		decision.UseHeal = value
	}
	if value, ok := parseOptionalInt(raw["poison_target_seat"]); ok {
		decision.PoisonTargetSeat = &value
	}
	if decision.Speech == "" && decision.TargetSeat == nil && !decision.UseHeal && decision.PoisonTargetSeat == nil {
		return Decision{}, fmt.Errorf("empty decision")
	}
	return decision, nil
}

func parseOptionalInt(value any) (int, bool) {
	switch v := value.(type) {
	case float64:
		if int(v) < 0 {
			return 0, false
		}
		return int(v), true
	case int:
		if v < 0 {
			return 0, false
		}
		return v, true
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, false
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, false
		}
		if parsed < 0 {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func parseOptionalBool(value any) (bool, bool) {
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true":
			return true, true
		case "false":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func normalizeDecision(decision Decision) Decision {
	decision.Speech = strings.TrimSpace(decision.Speech)
	decision.Reason = strings.TrimSpace(decision.Reason)
	return decision
}
