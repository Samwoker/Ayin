package config

import (
	"strings"
	"testing"
)

func TestLoadFromRequiresValidServiceURLs(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		wantErr string
	}{
		{
			name:    "missing database URL",
			env:     map[string]string{"REDIS_URL": "redis://localhost:6379"},
			wantErr: "DATABASE_URL",
		},
		{
			name:    "missing redis URL",
			env:     map[string]string{"DATABASE_URL": "postgres://localhost/triage"},
			wantErr: "REDIS_URL",
		},
		{
			name: "invalid URL scheme",
			env: map[string]string{
				"DATABASE_URL": "http://localhost/triage",
				"REDIS_URL":    "redis://localhost:6379",
			},
			wantErr: "DATABASE_URL",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadFrom(func(key string) string { return test.env[key] })
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("LoadFrom() error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestLoadFromDefaultsHTTPAddress(t *testing.T) {
	config, err := LoadFrom(func(key string) string {
		return map[string]string{
			"DATABASE_URL": "postgres://localhost/triage",
			"REDIS_URL":    "redis://localhost:6379",
		}[key]
	})
	if err != nil {
		t.Fatalf("LoadFrom() error = %v", err)
	}
	if config.HTTPAddr != ":8080" {
		t.Fatalf("HTTPAddr = %q, want %q", config.HTTPAddr, ":8080")
	}
}