package constants_test

import (
	"testing"

	"github.com/archguard/project/shared/constants"
)

func TestIsOpenAIReasoningModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  bool
	}{
		{"o1", "o1", true},
		{"o1-mini", "o1-mini", true},
		{"o1-preview", "o1-preview", true},
		{"o3", "o3", true},
		{"o3-mini", "o3-mini", true},
		{"o4-mini", "o4-mini", true},
		{"gpt-5", "gpt-5", true},
		{"gpt-5-thinking", "gpt-5-thinking", true},
		{"gpt-4o", "gpt-4o", false},
		{"gpt-4o-mini", "gpt-4o-mini", false},
		{"gpt-4-turbo", "gpt-4-turbo", false},
		{"chatgpt-4o-latest", "chatgpt-4o-latest", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := constants.IsOpenAIReasoningModel(tt.model); got != tt.want {
				t.Errorf("IsOpenAIReasoningModel(%q) = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestIsSensitivePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{".env", true},
		{"src/.env", true},
		{"secrets/key.txt", true},
		{"secrets/nested/token", true},
		{"cert.pem", true},
		{"config/server.key", true},
		{"src/App/Service.php", false},
		{"src/main.go", false},
		{"README.md", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := constants.IsSensitivePath(tt.path); got != tt.want {
				t.Errorf("IsSensitivePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
