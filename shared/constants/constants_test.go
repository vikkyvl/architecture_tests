package constants_test

import (
	"testing"

	"github.com/archguard/project/shared/constants"
)

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
