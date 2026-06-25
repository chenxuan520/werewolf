package config

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

type PresetFile struct {
	Presets []Preset `yaml:"presets"`
}

type Preset struct {
	ID               string            `yaml:"id" json:"id"`
	Name             string            `yaml:"name" json:"name"`
	Style            string            `yaml:"style,omitempty" json:"style,omitempty"`
	Persona          string            `yaml:"persona,omitempty" json:"persona,omitempty"`
	Endpoint         string            `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Token            string            `yaml:"token,omitempty" json:"-"`
	Model            string            `yaml:"model,omitempty" json:"model,omitempty"`
	SystemPrompt     string            `yaml:"system_prompt,omitempty" json:"systemPrompt,omitempty"`
	StructuredOutput string            `yaml:"structured_output,omitempty" json:"structuredOutput,omitempty"`
	MaxTokens        int               `yaml:"max_tokens,omitempty" json:"maxTokens,omitempty"`
	ExtraHeaders     map[string]string `yaml:"extra_headers,omitempty" json:"-"`
	ExtraBody        map[string]any    `yaml:"extra_body,omitempty" json:"-"`
}

const (
	StructuredOutputToolCall   = "tool_call"
	StructuredOutputJSONObject = "json_object"
	StructuredOutputNone       = "none"
)

func LoadPresets(path string) ([]Preset, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read presets: %w", err)
	}
	var file PresetFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse presets yaml: %w", err)
	}
	if len(file.Presets) == 0 {
		return nil, fmt.Errorf("no presets configured")
	}
	seen := map[string]struct{}{}
	for i := range file.Presets {
		preset := &file.Presets[i]
		preset.ID = strings.TrimSpace(preset.ID)
		preset.Name = strings.TrimSpace(preset.Name)
		preset.Style = strings.ToLower(strings.TrimSpace(preset.Style))
		preset.Persona = strings.TrimSpace(preset.Persona)
		preset.Endpoint = strings.TrimSpace(preset.Endpoint)
		preset.Token = strings.TrimSpace(preset.Token)
		preset.Model = strings.TrimSpace(preset.Model)
		preset.SystemPrompt = strings.TrimSpace(preset.SystemPrompt)
		if preset.ID == "" || preset.Name == "" {
			return nil, fmt.Errorf("preset %d: missing id or name", i+1)
		}
		normalized, err := normalizeStructuredOutput(preset.StructuredOutput)
		if err != nil {
			return nil, fmt.Errorf("preset %d: %w", i+1, err)
		}
		preset.StructuredOutput = normalized
		if preset.Style == "" {
			preset.Style = "steady"
		}
		if _, ok := seen[preset.ID]; ok {
			return nil, fmt.Errorf("preset %d: duplicate id %q", i+1, preset.ID)
		}
		seen[preset.ID] = struct{}{}
	}
	return file.Presets, nil
}

func (p Preset) UsesLLM() bool {
	return strings.TrimSpace(p.Endpoint) != "" && strings.TrimSpace(p.Token) != "" && strings.TrimSpace(p.Model) != ""
}

func (p Preset) StructuredOutputMode() string {
	if p.StructuredOutput == "" {
		return StructuredOutputToolCall
	}
	return p.StructuredOutput
}

func normalizeStructuredOutput(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "":
		return StructuredOutputToolCall, nil
	case "tool_call", "tool_calls", "tool", "tools":
		return StructuredOutputToolCall, nil
	case "json_object", "json", "jsonobject":
		return StructuredOutputJSONObject, nil
	case "none", "off", "disabled", "false":
		return StructuredOutputNone, nil
	default:
		return "", fmt.Errorf("invalid structured_output %q", value)
	}
}
