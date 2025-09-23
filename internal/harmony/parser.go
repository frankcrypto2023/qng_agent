package harmony

import (
	"regexp"
	"strings"
)

// HarmonyTag represents the different types of Harmony metadata tags
type HarmonyTag string

const (
	TagChannel HarmonyTag = "channel"
	TagMessage HarmonyTag = "message"
	TagStart   HarmonyTag = "start"
	TagEnd     HarmonyTag = "end"
)

// HarmonyMessage represents a parsed Harmony format message
type HarmonyMessage struct {
	Role    string `json:"role,omitempty"`
	Channel string `json:"channel,omitempty"`
	Content string `json:"content"`
}

// HarmonyParser handles parsing and filtering of Harmony format content
type HarmonyParser struct {
	// Compiled regex patterns for efficiency
	harmonyBlockPattern *regexp.Regexp
	standalonePattern   *regexp.Regexp
	cleanupPattern      *regexp.Regexp
}

// NewHarmonyParser creates a new Harmony parser with compiled regex patterns
func NewHarmonyParser() *HarmonyParser {
	return &HarmonyParser{
		// Pattern to match complete harmony blocks: <|tag|>content<|end|>
		harmonyBlockPattern: regexp.MustCompile(`(?s)<\|(channel|message|start|end)\|>.*?<\|end\|>`),

		// Pattern to match standalone harmony tags: <|tag|>content (without closing)
		standalonePattern: regexp.MustCompile(`<\|(channel|message|start|end)\|>[^<]*`),

		// Pattern for cleanup - multiple consecutive newlines
		cleanupPattern: regexp.MustCompile(`\n\s*\n\s*\n`),
	}
}

// FilterMetadata removes all Harmony metadata tags and returns clean content
func (p *HarmonyParser) FilterMetadata(content string) string {
	if content == "" {
		return content
	}

	// First, try to extract content from the last message block
	// Pattern: <|message|>content<|end|> or <|message|>content (without end)
	messagePattern := regexp.MustCompile(`<\|message\|>([^<]*?)(?:<\|end\|>|$)`)
	messageMatches := messagePattern.FindAllStringSubmatch(content, -1)

	if len(messageMatches) > 0 {
		// Return the last message content
		lastMatch := messageMatches[len(messageMatches)-1]
		if len(lastMatch) > 1 {
			return strings.TrimSpace(lastMatch[1])
		}
	}

	// Fallback: remove all harmony tags and return what's left
	// Remove complete harmony blocks
	filtered := p.harmonyBlockPattern.ReplaceAllString(content, "")

	// Remove standalone harmony tags
	filtered = p.standalonePattern.ReplaceAllString(filtered, "")

	// Clean up extra whitespace
	filtered = p.cleanupPattern.ReplaceAllString(filtered, "\n\n")
	filtered = strings.TrimSpace(filtered)

	return filtered
}

// ParseMessage parses a Harmony format string and extracts structured message data
func (p *HarmonyParser) ParseMessage(content string) (*HarmonyMessage, error) {
	// First filter out metadata to get clean content
	cleanContent := p.FilterMetadata(content)

	message := &HarmonyMessage{
		Content: cleanContent,
	}

	// Extract role and channel information from original content if present
	// Look for the last occurrence of role and channel (closest to the final message)
	roleMatch := regexp.MustCompile(`<\|start\|>([^<]*?)<\|channel\|>`)
	roleMatches := roleMatch.FindAllStringSubmatch(content, -1)
	if len(roleMatches) > 0 {
		// Use the last role found (closest to the final message)
		lastRole := roleMatches[len(roleMatches)-1]
		if len(lastRole) > 1 {
			message.Role = strings.TrimSpace(lastRole[1])
		}
	}

	channelMatch := regexp.MustCompile(`<\|channel\|>([^<]*?)<\|message\|>`)
	channelMatches := channelMatch.FindAllStringSubmatch(content, -1)
	if len(channelMatches) > 0 {
		// Use the last channel found (closest to the final message)
		lastChannel := channelMatches[len(channelMatches)-1]
		if len(lastChannel) > 1 {
			message.Channel = strings.TrimSpace(lastChannel[1])
		}
	}

	return message, nil
}

// IsHarmonyFormat checks if the content contains Harmony format tags
func (p *HarmonyParser) IsHarmonyFormat(content string) bool {
	return p.harmonyBlockPattern.MatchString(content) || p.standalonePattern.MatchString(content)
}

// ExtractAllTags extracts all Harmony tags from content
func (p *HarmonyParser) ExtractAllTags(content string) []HarmonyTag {
	var tags []HarmonyTag

	// Find all tag occurrences
	tagPattern := regexp.MustCompile(`<\|(channel|message|start|end)\|>`)
	matches := tagPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) > 1 {
			tags = append(tags, HarmonyTag(match[1]))
		}
	}

	return tags
}

// GetTagContent extracts content between specific tags
func (p *HarmonyParser) GetTagContent(content string, tag HarmonyTag) string {
	pattern := regexp.MustCompile(`<\|` + string(tag) + `\|>([^<]*?)<\|end\|>`)
	matches := pattern.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}
