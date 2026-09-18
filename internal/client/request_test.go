package client

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPhaseClientDoSuccess(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
	}{
		{
			name:       "OK",
			statusCode: http.StatusOK,
			body:       `{"id":"app-123"}`,
		},
		{
			name:       "Created",
			statusCode: http.StatusCreated,
			body:       `{"id":"app-123"}`,
		},
		{
			name:       "No Content",
			statusCode: http.StatusNoContent,
			body:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)

				if tt.body != "" {
					_, _ = w.Write([]byte(tt.body))
				}
			}))
			defer server.Close()

			c := &PhaseClient{
				HostURL:    server.URL,
				HTTPClient: server.Client(),
				Token:      "test-token",
				TokenType:  "user",
			}

			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			body, err := c.Do(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(body) != tt.body {
				t.Errorf("expected body %q, got %q", tt.body, string(body))
			}
		})
	}
}

func TestPhaseClientDoAPIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		status     string
		body       string
	}{
		{
			name:       "Bad Request",
			statusCode: http.StatusBadRequest,
			status:     "400 Bad Request",
			body:       `{"message":"invalid request"}`,
		},
		{
			name:       "Unauthorized",
			statusCode: http.StatusUnauthorized,
			status:     "401 Unauthorized",
			body:       `{"message":"unauthorized"}`,
		},
		{
			name:       "Not Found",
			statusCode: http.StatusNotFound,
			status:     "404 Not Found",
			body:       `{"message":"application not found"}`,
		},
		{
			name:       "Conflict",
			statusCode: http.StatusConflict,
			status:     "409 Conflict",
			body:       `{"message":"application already exists"}`,
		},
		{
			name:       "Internal Server Error",
			statusCode: http.StatusInternalServerError,
			status:     "500 Internal Server Error",
			body:       `{"message":"internal server error"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			c := &PhaseClient{
				HostURL:    server.URL,
				HTTPClient: server.Client(),
				Token:      "test-token",
				TokenType:  "user",
			}

			req, err := http.NewRequest(http.MethodGet, server.URL, nil)
			if err != nil {
				t.Fatalf("failed to create request: %v", err)
			}

			body, err := c.Do(req)

			if body != nil {
				t.Errorf("expected nil body, got %q", body)
			}

			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected *APIError, got %T: %v", err, err)
			}

			if apiErr.StatusCode != tt.statusCode {
				t.Errorf(
					"expected status code %d, got %d",
					tt.statusCode,
					apiErr.StatusCode,
				)
			}

			if apiErr.Status != tt.status {
				t.Errorf(
					"expected status %q, got %q",
					tt.status,
					apiErr.Status,
				)
			}

			if string(apiErr.Body) != tt.body {
				t.Errorf(
					"expected body %q, got %q",
					tt.body,
					string(apiErr.Body),
				)
			}
		})
	}
}

func TestPhaseClientDoSetsHeaders(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf(
				"expected Content-Type application/json, got %q",
				got,
			)
		}

		if got := r.Header.Get("Authorization"); got != "Bearer user test-token" {
			t.Errorf(
				"expected Authorization %q, got %q",
				"Bearer user test-token",
				got,
			)
		}

		if got := r.Header.Get("User-Agent"); got == "" {
			t.Error("expected User-Agent header to be set")
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c := &PhaseClient{
		HostURL:    server.URL,
		HTTPClient: server.Client(),
		Token:      "test-token",
		TokenType:  "user",
	}

	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	_, err = c.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPhaseClientDoHTTPError(t *testing.T) {
	c := &PhaseClient{
		HTTPClient: http.DefaultClient,
		Token:      "test-token",
		TokenType:  "user",
	}

	req, err := http.NewRequest(
		http.MethodGet,
		"http://127.0.0.1:1",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	body, err := c.Do(req)

	if body != nil {
		t.Errorf("expected nil body, got %q", body)
	}

	if err == nil {
		t.Fatal("expected HTTP error, got nil")
	}

	var apiErr *APIError
	if errors.As(err, &apiErr) {
		t.Fatalf("expected transport error, got APIError: %v", err)
	}
}

func TestPhaseClientDoResponseBodyReadError(t *testing.T) {
	client := &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Status:     "200 OK",
				Body:       io.NopCloser(errorReader{}),
				Header:     make(http.Header),
			}, nil
		}),
	}

	c := &PhaseClient{
		HTTPClient: client,
		Token:      "test-token",
		TokenType:  "user",
	}

	req, err := http.NewRequest(
		http.MethodGet,
		"http://example.com",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	body, err := c.Do(req)

	if body != nil {
		t.Errorf("expected nil body, got %q", body)
	}

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "error reading response body") {
		t.Errorf(
			"expected body read error, got %q",
			err.Error(),
		)
	}
}

func TestPhaseClientSetHeaders(t *testing.T) {
	c := &PhaseClient{
		Token:     "secret-token",
		TokenType: "service",
	}

	req, err := http.NewRequest(
		http.MethodGet,
		"http://example.com",
		nil,
	)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	c.setHeaders(req)

	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf(
			"expected Content-Type application/json, got %q",
			got,
		)
	}

	expectedAuth := "Bearer service secret-token"
	if got := req.Header.Get("Authorization"); got != expectedAuth {
		t.Errorf(
			"expected Authorization %q, got %q",
			expectedAuth,
			got,
		)
	}

	if got := req.Header.Get("User-Agent"); got == "" {
		t.Error("expected User-Agent to be set")
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) {
	return 0, errors.New("failed to read body")
}
