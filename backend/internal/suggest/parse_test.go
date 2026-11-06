package suggest

import (
	"strings"
	"testing"
	"time"
)

func TestParseSuggestionCleansModelReply(t *testing.T) {
	raw := "Sure! ```json\n{\"title\": \"  Nintendo   Game Boy Color, Blue \", \"description\": \"A handheld console in blue.\", " +
		"\"tags\": [\"Handheld\", \"#Retro\", \"vintage games\", \"retro\", \"\", \"a b c d e f g h i j k l\"]}\n``` hope that helps"

	s, err := parseSuggestion(raw)
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "Nintendo Game Boy Color, Blue" {
		t.Errorf("title = %q", s.Title)
	}
	wantTags := []string{"handheld", "retro", "vintage_games", "a_b_c_d_e_f_g_h_i_j_k_l"}
	if strings.Join(s.Tags, ",") != strings.Join(wantTags, ",") {
		t.Errorf("tags = %v, want %v", s.Tags, wantTags)
	}
	for _, tag := range s.Tags {
		if !strings.Contains(s.Description, "#"+tag) {
			t.Errorf("description %q is missing #%s, so the tag would not be stored", s.Description, tag)
		}
	}
}

func TestParseSuggestionCapsTagCount(t *testing.T) {
	raw := `{"title": "x", "description": "", "tags": ["a","b","c","d","e","f","g","h","i","j","k","l"]}`
	s, err := parseSuggestion(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Tags) != maxTags {
		t.Errorf("got %d tags, want at most %d", len(s.Tags), maxTags)
	}
}

func TestParseSuggestionFallsBackToUnknownTitle(t *testing.T) {
	s, err := parseSuggestion(`{"title": "   ", "description": "", "tags": []}`)
	if err != nil {
		t.Fatal(err)
	}
	if s.Title != "Unknown item" {
		t.Errorf("title = %q", s.Title)
	}
}

func TestParseSuggestionRejectsNonJSON(t *testing.T) {
	for _, raw := range []string{"", "I cannot tell what this is.", "{not json}"} {
		if _, err := parseSuggestion(raw); err == nil {
			t.Errorf("parseSuggestion(%q) succeeded, want an error", raw)
		}
	}
}

func TestDailyLimitResetsEachDay(t *testing.T) {
	l := newDailyLimit(2)
	day1 := time.Date(2026, 11, 5, 10, 0, 0, 0, time.UTC)
	if !l.allow("u", day1) || !l.allow("u", day1) {
		t.Fatal("first two calls should be allowed")
	}
	if l.allow("u", day1.Add(time.Hour)) {
		t.Error("third call on the same day should be refused")
	}
	if !l.allow("u", day1.Add(24*time.Hour)) {
		t.Error("the limit should reset on the next day")
	}
	if !l.allow("other", day1) {
		t.Error("limits are per user")
	}
}
