package xss

import (
	"regexp"
	"strings"
)

var (
	// stripTagsRegex removes all HTML tags
	stripTagsRegex = regexp.MustCompile(`<[^>]*>`)
	// multiSpaceRegex collapses multiple whitespace
	multiSpaceRegex = regexp.MustCompile(`\s+`)
	// Dangerous script patterns
	scriptPattern = regexp.MustCompile(`(?i)<\s*script[^>]*>.*?<\s*/\s*script\s*>`)
	iframePattern = regexp.MustCompile(`(?i)<\s*iframe[^>]*>.*?<\s*/\s*iframe\s*>`)
	eventPattern  = regexp.MustCompile(`(?i)\s+on\w+\s*=`)
)

// SanitizeMarkdown removes dangerous HTML from markdown content
// while preserving safe markdown formatting.
func SanitizeMarkdown(content string) string {
	s := content
	s = scriptPattern.ReplaceAllString(s, "")
	s = iframePattern.ReplaceAllString(s, "")
	s = eventPattern.ReplaceAllString(s, "")
	return s
}

// StripHTML converts content to plain text by removing all HTML tags
// and collapsing whitespace. Used for search indexing.
func StripHTML(content string) string {
	s := stripTagsRegex.ReplaceAllString(content, " ")
	s = multiSpaceRegex.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// TruncateText truncates text to a maximum length, adding ellipsis if needed.
func TruncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	if maxLen > 3 {
		return text[:maxLen-3] + "..."
	}
	return text[:maxLen]
}
