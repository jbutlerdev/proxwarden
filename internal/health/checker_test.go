package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jbutlerdev/proxwarden/internal/config"
	"github.com/sirupsen/logrus"
)

func TestChecker_RunHealthCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel) // Suppress logs during tests
	checker := NewChecker(logger)

	tests := []struct {
		name          string
		check         config.HealthCheck
		expectSuccess bool
	}{
		{
			name: "http success",
			check: config.HealthCheck{
				Type:    "http",
				Target:  "httpbin.org",
				Port:    80,
				Path:    "/status/200",
				Timeout: 10 * time.Second,
			},
			expectSuccess: true,
		},
		{
			name: "http failure",
			check: config.HealthCheck{
				Type:    "http",
				Target:  "httpbin.org",
				Port:    80,
				Path:    "/status/500",
				Timeout: 10 * time.Second,
			},
			expectSuccess: false,
		},
		{
			name: "tcp success - connect to http server",
			check: config.HealthCheck{
				Type:    "tcp",
				Target:  "google.com",
				Port:    80,
				Timeout: 5 * time.Second,
			},
			expectSuccess: true,
		},
		{
			name: "tcp failure - invalid port",
			check: config.HealthCheck{
				Type:    "tcp",
				Target:  "google.com",
				Port:    99999,
				Timeout: 5 * time.Second,
			},
			expectSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result := checker.RunHealthCheck(ctx, tt.check)

			if result.Success != tt.expectSuccess {
				t.Errorf("Expected success=%v, got success=%v, error=%v", 
					tt.expectSuccess, result.Success, result.Error)
			}

			if result.Type != tt.check.Type {
				t.Errorf("Expected type=%s, got type=%s", tt.check.Type, result.Type)
			}

			if result.Target != tt.check.Target {
				t.Errorf("Expected target=%s, got target=%s", tt.check.Target, result.Target)
			}

			if result.Duration <= 0 {
				t.Error("Expected positive duration")
			}

			if result.Timestamp.IsZero() {
				t.Error("Expected non-zero timestamp")
			}
		})
	}
}

func TestChecker_HTTPCheck(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	checker := NewChecker(logger)

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/ok":
			w.WriteHeader(http.StatusOK)
		case "/not-found":
			w.WriteHeader(http.StatusNotFound)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"success response", "/ok", true},
		{"not found response", "/not-found", false},
		{"server error response", "/error", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// Extract host and port from server URL
			host := "127.0.0.1"
			port := extractPortFromURL(server.URL)
			success, err := checker.httpCheck(ctx, "http", host, port, tt.path)

			if success != tt.expected {
				t.Errorf("Expected %v, got %v, error: %v", tt.expected, success, err)
			}
		})
	}
}

func TestChecker_ContextTimeout(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.FatalLevel)
	checker := NewChecker(logger)

	// Create a context that times out quickly
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	check := config.HealthCheck{
		Type:    "tcp",
		Target:  "google.com",
		Port:    80,
		Timeout: 1 * time.Second, // This should be ignored due to context timeout
	}

	result := checker.RunHealthCheck(ctx, check)

	if result.Success {
		t.Error("Expected failure due to context timeout")
	}

	if result.Error == nil {
		t.Error("Expected error due to context timeout")
	}
}

// Helper function to extract port from server URL
func extractPortFromURL(url string) int {
	// This is a simple helper for the test server
	// Extract port from URL like "http://127.0.0.1:44321"
	if len(url) > 17 { // http://127.0.0.1:
		// Find the port part after the last colon
		parts := strings.Split(url, ":")
		if len(parts) >= 3 {
			// Try to parse the port number
			if port, err := strconv.Atoi(parts[len(parts)-1]); err == nil {
				return port
			}
		}
	}
	return 80 // Default port
}