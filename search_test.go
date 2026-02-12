package main

import (
	"testing"
)

func TestValidateCategory(t *testing.T) {
	valid := []string{"general", "news", "videos", "images", "music", "social media", "social-media", "social_media"}
	for _, cat := range valid {
		if !validateCategory(cat) {
			t.Errorf("validateCategory(%q) should be true", cat)
		}
	}

	invalid := []string{"invalid", "foo", ""}
	for _, cat := range invalid {
		if validateCategory(cat) {
			t.Errorf("validateCategory(%q) should be false", cat)
		}
	}
}

func TestNormalizeCategoryMain(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"social-media", "social media"},
		{"social+media", "social media"},
		{"social_media", "social media"},
		{"socialmedia", "social media"},
		{"news", "news"},
		{"general", "general"},
	}
	for _, tt := range tests {
		if got := normalizeCategory(tt.input); got != tt.want {
			t.Errorf("normalizeCategory(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidateTimeRange(t *testing.T) {
	valid := []string{"day", "week", "month", "year", "d", "w", "m", "y"}
	for _, tr := range valid {
		if !validateTimeRange(tr) {
			t.Errorf("validateTimeRange(%q) should be true", tr)
		}
	}

	invalid := []string{"invalid", "decade", "hour", ""}
	for _, tr := range invalid {
		if validateTimeRange(tr) {
			t.Errorf("validateTimeRange(%q) should be false", tr)
		}
	}
}

func TestExpandTimeRange(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"d", "day"},
		{"w", "week"},
		{"m", "month"},
		{"y", "year"},
		{"day", "day"},
		{"week", "week"},
		{"unknown", "unknown"},
	}
	for _, tt := range tests {
		if got := expandTimeRange(tt.input); got != tt.want {
			t.Errorf("expandTimeRange(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestValidEngineNames(t *testing.T) {
	names := validEngineNames()
	if names == "" {
		t.Error("validEngineNames() should not be empty")
	}
	// Should contain all three engines
	for _, engine := range []string{"searxng", "brave", "tavily"} {
		if !contains(names, engine) {
			t.Errorf("validEngineNames() should contain %q, got %q", engine, names)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
