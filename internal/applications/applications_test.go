package applications

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/phasehq/terraform-provider/internal/client"
)

func newTestClient(server *httptest.Server) *client.PhaseClient {
	return &client.PhaseClient{
		HostURL:    server.URL,
		HTTPClient: server.Client(),
		Token:      "test-token",
		TokenType:  "User",
	}
}

func testApplication() Application {
	return Application{
		ID:          "app-123",
		Name:        "test-application",
		Description: "test description",
		SseEnable:   true,
	}
}

func TestCreateApplication(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedID   string
		expectedErr  string
	}{
		{
			name:         "success",
			statusCode:   http.StatusCreated,
			responseBody: `{"id":"app-123","name":"test-application","description":"test description","sseEnabled":true}`,
			expectedID:   "app-123",
		},
		{
			name:         "api error",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"message":"invalid application"}`,
			expectedErr:  "failed to create application",
		},
		{
			name:         "invalid json response",
			statusCode:   http.StatusCreated,
			responseBody: `invalid-json`,
			expectedErr:  "invalid character",
		},
		{
			name:         "missing application id",
			statusCode:   http.StatusCreated,
			responseBody: `{"name":"test-application"}`,
			expectedErr:  "create application response did not contain an application ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected method POST, got %s", r.Method)
				}

				if r.URL.Path != "/v1/apps/" {
					t.Errorf("expected path /v1/apps/, got %s", r.URL.Path)
				}

				var body CreateApplicationRequest
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				if tt.name == "success" {
					if body.Name != "test-application" {
						t.Errorf("expected name test-application, got %s", body.Name)
					}

					if body.Description == nil {
						t.Fatal("expected description, got nil")
					}

					if *body.Description != "test description" {
						t.Errorf(
							"expected description test description, got %s",
							*body.Description,
						)
					}
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c := newTestClient(server)

			description := "test description"

			app := CreateApplicationRequest{
				Name:        "test-application",
				Description: &description,
			}

			got, err := CreateApplication(c, CreateApplicationRequest{
				Name:        app.Name,
				Description: app.Description,
			})

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedErr)
				}

				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf(
						"expected error containing %q, got %q",
						tt.expectedErr,
						err.Error(),
					)
				}

				var apiErr *client.APIError
				if tt.name == "api error" && !errors.As(err, &apiErr) {
					t.Fatal("expected error to wrap *client.APIError")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got == nil {
				t.Fatal("expected application, got nil")
			}

			if got.ID != tt.expectedID {
				t.Errorf("expected ID %s, got %s", tt.expectedID, got.ID)
			}

			if got.Name != "test-application" {
				t.Errorf("expected name test-application, got %s", got.Name)
			}

			if got.Description != "test description" {
				t.Errorf(
					"expected description test description, got %s",
					got.Description,
				)
			}
		})
	}
}

func TestReadApplication(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedID   string
		expectedErr  string
	}{
		{
			name:         "success",
			statusCode:   http.StatusOK,
			responseBody: `{"id":"app-123","name":"test-application","description":"test description","sseEnabled":true}`,
			expectedID:   "app-123",
		},
		{
			name:         "api error",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message":"application not found"}`,
			expectedErr:  "failed to read application",
		},
		{
			name:         "invalid json response",
			statusCode:   http.StatusOK,
			responseBody: `invalid-json`,
			expectedErr:  "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected method GET, got %s", r.Method)
				}

				if r.URL.Path != "/v1/apps/app-123/" {
					t.Errorf(
						"expected path /v1/apps/app-123/, got %s",
						r.URL.Path,
					)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c := newTestClient(server)

			got, err := ReadApplication(c, "app-123")

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedErr)
				}

				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf(
						"expected error containing %q, got %q",
						tt.expectedErr,
						err.Error(),
					)
				}

				var apiErr *client.APIError
				if tt.name == "api error" && !errors.As(err, &apiErr) {
					t.Fatal("expected error to wrap *client.APIError")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got == nil {
				t.Fatal("expected application, got nil")
			}

			if got.ID != tt.expectedID {
				t.Errorf("expected ID %s, got %s", tt.expectedID, got.ID)
			}

			if got.Name != "test-application" {
				t.Errorf("expected name test-application, got %s", got.Name)
			}

			if got.Description != "test description" {
				t.Errorf(
					"expected description test description, got %s",
					got.Description,
				)
			}

			if !got.SseEnable {
				t.Error("expected SseEnable to be true")
			}
		})
	}
}

func TestListApplications(t *testing.T) {
	tests := []struct {
		name          string
		statusCode    int
		responseBody  string
		expectedCount int
		expectedErr   string
	}{
		{
			name:          "success",
			statusCode:    http.StatusOK,
			responseBody:  `{"data":[{"id":"app-1","name":"app-one"},{"id":"app-2","name":"app-two"}]}`,
			expectedCount: 2,
		},
		{
			name:          "empty list",
			statusCode:    http.StatusOK,
			responseBody:  `{"data":[]}`,
			expectedCount: 0,
		},
		{
			name:         "api error",
			statusCode:   http.StatusInternalServerError,
			responseBody: `{"message":"internal server error"}`,
			expectedErr:  "failed to list application",
		},
		{
			name:         "invalid json response",
			statusCode:   http.StatusOK,
			responseBody: `invalid-json`,
			expectedErr:  "invalid character",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected method GET, got %s", r.Method)
				}

				if r.URL.Path != "/v1/apps/" {
					t.Errorf(
						"expected path /v1/apps/, got %s",
						r.URL.Path,
					)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c := newTestClient(server)

			got, err := ListApplications(c)

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedErr)
				}

				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf(
						"expected error containing %q, got %q",
						tt.expectedErr,
						err.Error(),
					)
				}

				var apiErr *client.APIError
				if tt.name == "api error" && !errors.As(err, &apiErr) {
					t.Fatal("expected error to wrap *client.APIError")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != tt.expectedCount {
				t.Fatalf(
					"expected %d applications, got %d",
					tt.expectedCount,
					len(got),
				)
			}

			if tt.name == "success" {
				if got[0].ID != "app-1" {
					t.Errorf("expected first ID app-1, got %s", got[0].ID)
				}

				if got[1].ID != "app-2" {
					t.Errorf("expected second ID app-2, got %s", got[1].ID)
				}
			}
		})
	}
}

func TestUpdateApplication(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedID   string
		expectedErr  string
	}{
		{
			name:         "success",
			statusCode:   http.StatusOK,
			responseBody: `{"id":"app-123","name":"updated-application","description":"updated description","sseEnabled":true}`,
			expectedID:   "app-123",
		},
		{
			name:         "api error",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message":"application not found"}`,
			expectedErr:  "failed to update application",
		},
		{
			name:         "invalid json response",
			statusCode:   http.StatusOK,
			responseBody: `invalid-json`,
			expectedErr:  "invalid character",
		},
		{
			name:         "nil application response",
			statusCode:   http.StatusOK,
			responseBody: `null`,
			expectedErr:  "no app updated",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPut {
					t.Errorf("expected method PUT, got %s", r.Method)
				}

				if r.URL.Path != "/v1/apps/app-123/" {
					t.Errorf(
						"expected path /v1/apps/app-123/, got %s",
						r.URL.Path,
					)
				}

				var body Application
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				if body.ID != "app-123" {
					t.Errorf("expected ID app-123, got %s", body.ID)
				}

				if body.Name != "test-application" {
					t.Errorf(
						"expected name test-application, got %s",
						body.Name,
					)
				}

				if body.Description != "test description" {
					t.Errorf(
						"expected description test description, got %s",
						body.Description,
					)
				}

				if !body.SseEnable {
					t.Error("expected SseEnable to be true")
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			c := newTestClient(server)

			got, err := UpdateApplication(c, "app-123", testApplication())

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedErr)
				}

				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf(
						"expected error containing %q, got %q",
						tt.expectedErr,
						err.Error(),
					)
				}

				var apiErr *client.APIError
				if tt.name == "api error" && !errors.As(err, &apiErr) {
					t.Fatal("expected error to wrap *client.APIError")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got == nil {
				t.Fatal("expected application, got nil")
			}

			if got.ID != tt.expectedID {
				t.Errorf("expected ID %s, got %s", tt.expectedID, got.ID)
			}

			if got.Name != "updated-application" {
				t.Errorf(
					"expected name updated-application, got %s",
					got.Name,
				)
			}
		})
	}
}

func TestDeleteApplication(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedErr  string
	}{
		{
			name:       "success",
			statusCode: http.StatusNoContent,
		},
		{
			name:         "api error",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message":"application not found"}`,
			expectedErr:  "failed to delete application",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodDelete {
					t.Errorf("expected method DELETE, got %s", r.Method)
				}

				if r.URL.Path != "/v1/apps/app-123/" {
					t.Errorf(
						"expected path /v1/apps/app-123/, got %s",
						r.URL.Path,
					)
				}

				w.WriteHeader(tt.statusCode)

				if tt.responseBody != "" {
					_, _ = w.Write([]byte(tt.responseBody))
				}
			}))
			defer server.Close()

			c := newTestClient(server)

			err := DeleteApplication(c, "app-123")

			if tt.expectedErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.expectedErr)
				}

				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Fatalf(
						"expected error containing %q, got %q",
						tt.expectedErr,
						err.Error(),
					)
				}

				var apiErr *client.APIError
				if !errors.As(err, &apiErr) {
					t.Fatal("expected error to wrap *client.APIError")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
