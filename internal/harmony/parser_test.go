package harmony

import (
	"testing"
)

func TestHarmonyParser_FilterMetadata(t *testing.T) {
	parser := NewHarmonyParser()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Complex harmony format",
			input:    `<|channel|>analysis<|message|>We have a user conversation: "你好". It's Chinese greeting. The assistant should respond in Chinese. No specific query about blockchain. So just respond friendly.<|end|><|start|>assistant<|channel|>final<|message|>你好！😊`,
			expected: `你好！😊`,
		},
		{
			name:     "Simple message",
			input:    `<|start|>assistant<|channel|>final<|message|>Hello world<|end|>`,
			expected: `Hello world`,
		},
		{
			name:     "Multiple blocks",
			input:    `<|channel|>test<|message|>First message<|end|><|start|>user<|message|>Second message<|end|>`,
			expected: `Second message`,
		},
		{
			name:     "Empty content",
			input:    `<|channel|>empty<|message|><|end|>`,
			expected: ``,
		},
		{
			name:     "No harmony tags",
			input:    `This is just plain text`,
			expected: `This is just plain text`,
		},
		{
			name:     "Mixed content",
			input:    `Before <|start|>assistant<|message|>In the middle<|end|> After`,
			expected: `In the middle`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.FilterMetadata(tt.input)
			if result != tt.expected {
				t.Errorf("FilterMetadata() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHarmonyParser_ParseMessage(t *testing.T) {
	parser := NewHarmonyParser()

	tests := []struct {
		name            string
		input           string
		expectedRole    string
		expectedChannel string
		expectedContent string
	}{
		{
			name:            "Complete message",
			input:           `<|start|>assistant<|channel|>final<|message|>你好！😊<|end|>`,
			expectedRole:    "assistant",
			expectedChannel: "final",
			expectedContent: "你好！😊",
		},
		{
			name:            "Analysis message",
			input:           `<|channel|>analysis<|message|>We have a user conversation: "你好". It's Chinese greeting.<|end|><|start|>assistant<|channel|>final<|message|>你好！😊`,
			expectedRole:    "assistant",
			expectedChannel: "final",
			expectedContent: "你好！😊",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message, err := parser.ParseMessage(tt.input)
			if err != nil {
				t.Errorf("ParseMessage() error = %v", err)
				return
			}

			if message.Role != tt.expectedRole {
				t.Errorf("ParseMessage() role = %q, want %q", message.Role, tt.expectedRole)
			}
			if message.Channel != tt.expectedChannel {
				t.Errorf("ParseMessage() channel = %q, want %q", message.Channel, tt.expectedChannel)
			}
			if message.Content != tt.expectedContent {
				t.Errorf("ParseMessage() content = %q, want %q", message.Content, tt.expectedContent)
			}
		})
	}
}

func TestHarmonyParser_IsHarmonyFormat(t *testing.T) {
	parser := NewHarmonyParser()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "Harmony format",
			input:    `<|start|>assistant<|message|>Hello<|end|>`,
			expected: true,
		},
		{
			name:     "Plain text",
			input:    `Just plain text`,
			expected: false,
		},
		{
			name:     "Partial harmony",
			input:    `<|start|>assistant`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.IsHarmonyFormat(tt.input)
			if result != tt.expected {
				t.Errorf("IsHarmonyFormat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestHarmonyParser_ExtractAllTags(t *testing.T) {
	parser := NewHarmonyParser()

	input := `<|start|>assistant<|channel|>final<|message|>Hello<|end|>`
	tags := parser.ExtractAllTags(input)

	expectedTags := []HarmonyTag{TagStart, TagChannel, TagMessage, TagEnd}

	if len(tags) != len(expectedTags) {
		t.Errorf("ExtractAllTags() returned %d tags, want %d", len(tags), len(expectedTags))
	}

	for i, tag := range tags {
		if tag != expectedTags[i] {
			t.Errorf("ExtractAllTags() tag[%d] = %q, want %q", i, tag, expectedTags[i])
		}
	}
}
