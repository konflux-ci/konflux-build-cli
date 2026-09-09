package common

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseQuayDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		// Valid durations
		{"1h", 1 * time.Hour, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"3d", 3 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"2w", 2 * 7 * 24 * time.Hour, false},
		{"52w", 52 * 7 * 24 * time.Hour, false},

		// Invalid durations
		{"", 0, true},
		{"h", 0, true},
		{"0h", 0, true},
		{"-1d", 0, true},
		{"2x", 0, true},
		{"abc", 0, true},
		{"2", 0, true},
		{"2.5w", 0, true},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ParseQuayDuration(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Errorf("ParseQuayDuration(%q) expected error, got %v", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseQuayDuration(%q) unexpected error: %v", tc.input, err)
				return
			}
			if got != tc.expected {
				t.Errorf("ParseQuayDuration(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

func TestIsQuayRegistry(t *testing.T) {
	tests := []struct {
		imageRef string
		expected bool
	}{
		{"quay.io/myorg/myrepo:tag", true},
		{"quay.io/myorg/myrepo", true},
		{"quay.io/myorg/myrepo@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855", true},
		{"docker.io/library/nginx:latest", false},
		{"registry.access.redhat.com/ubi8:latest", false},
		{"ghcr.io/owner/repo:tag", false},
		{"localhost:5000/myimage:tag", false},
		{"", false},
		{"invalid", false},
	}

	for _, tc := range tests {
		t.Run(tc.imageRef, func(t *testing.T) {
			got := IsQuayRegistry(tc.imageRef)
			if got != tc.expected {
				t.Errorf("IsQuayRegistry(%q) = %v, want %v", tc.imageRef, got, tc.expected)
			}
		})
	}
}

func TestSetTagExpiration(t *testing.T) {
	expiration := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	t.Run("successful API call", func(t *testing.T) {
		var capturedMethod, capturedPath, capturedAuth string
		var capturedBody map[string]any

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedMethod = r.Method
			capturedPath = r.URL.Path
			capturedAuth = r.Header.Get("Authorization")

			decoder := json.NewDecoder(r.Body)
			decoder.Decode(&capturedBody)

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{}`))
		}))
		defer server.Close()

		// Extract host from test server URL (strip https://)
		host := strings.TrimPrefix(server.URL, "https://")

		client := &quayAPIClient{
			registryDomain: host,
			authToken:      "dGVzdDp0ZXN0", // base64("test:test")
			httpClient:     server.Client(),
		}

		err := client.SetTagExpiration("myorg/myrepo", "v1.0", expiration)
		if err != nil {
			t.Fatalf("SetTagExpiration returned error: %v", err)
		}

		if capturedMethod != "PUT" {
			t.Errorf("Expected PUT method, got %s", capturedMethod)
		}
		if capturedPath != "/api/v1/repository/myorg/myrepo/tag/v1.0" {
			t.Errorf("Expected /api/v1/repository/myorg/myrepo/tag/v1.0, got %s", capturedPath)
		}
		if capturedAuth != "Basic dGVzdDp0ZXN0" {
			t.Errorf("Expected Basic auth, got %s", capturedAuth)
		}
		if exp, ok := capturedBody["expiration"]; !ok {
			t.Error("Expected expiration in body")
		} else if expFloat, ok := exp.(float64); !ok {
			t.Errorf("Expected expiration to be a number, got %T", exp)
		} else if int64(expFloat) != expiration.Unix() {
			t.Errorf("Expected expiration %d, got %v", expiration.Unix(), exp)
		}
	})

	t.Run("API returns error status", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte(`{"error": "permission denied"}`))
		}))
		defer server.Close()

		host := strings.TrimPrefix(server.URL, "https://")

		client := &quayAPIClient{
			registryDomain: host,
			authToken:      "dGVzdDp0ZXN0",
			httpClient:     server.Client(),
		}

		err := client.SetTagExpiration("myorg/myrepo", "v1.0", expiration)
		if err == nil {
			t.Fatal("Expected error for 403 response")
		}
		if !strings.Contains(err.Error(), "HTTP 403") {
			t.Errorf("Expected HTTP 403 in error, got: %v", err)
		}
	})

	t.Run("nested repository path", func(t *testing.T) {
		var capturedPath string

		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			capturedPath = r.URL.Path
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		host := strings.TrimPrefix(server.URL, "https://")

		client := &quayAPIClient{
			registryDomain: host,
			authToken:      "dGVzdDp0ZXN0",
			httpClient:     server.Client(),
		}

		err := client.SetTagExpiration("myorg/nested/repo", "latest", expiration)
		if err != nil {
			t.Fatalf("SetTagExpiration returned error: %v", err)
		}
		if capturedPath != "/api/v1/repository/myorg/nested/repo/tag/latest" {
			t.Errorf("Expected nested path, got %s", capturedPath)
		}
	})
}
