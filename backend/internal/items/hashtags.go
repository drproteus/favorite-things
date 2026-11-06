package items

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	maxTags      = 20
	maxTagLength = 40
)

// A hashtag starts the text or follows a character that can't be part of a word,
// so "C#" and "a&#b" are not tags. Migration 004 uses the same rule.
var hashtagPattern = regexp.MustCompile(`(?:^|[^\p{L}\p{N}_&/])#([\p{L}\p{N}_]+)`)

// hashtags returns the distinct lowercased hashtags in text, in order of first appearance.
func hashtags(text string) ([]string, error) {
	seen := map[string]bool{}
	tags := []string{}
	for _, m := range hashtagPattern.FindAllStringSubmatch(text, -1) {
		tag := strings.ToLower(m[1])
		if seen[tag] {
			continue
		}
		if utf8.RuneCountInString(tag) > maxTagLength {
			return nil, fmt.Errorf("hashtag #%s must be at most %d characters", tag, maxTagLength)
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	if len(tags) > maxTags {
		return nil, fmt.Errorf("at most %d hashtags are allowed", maxTags)
	}
	return tags, nil
}

// normalizeTag turns a filter value such as "#Retro" into "retro".
func normalizeTag(t string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(t), "#"))
}
