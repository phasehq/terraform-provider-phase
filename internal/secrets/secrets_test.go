package secrets

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
		TokenType:  "user",
	}
}

func TestCreateSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}

		if r.URL.Path != "/v1/secrets/" {
			t.Errorf("expected /v1/secrets/, got %s", r.URL.Path)
		}

		if got := r.URL.Query().Get("app_id"); got != "app-123" {
			t.Errorf("expected app_id=app-123, got %s", got)
		}

		if got := r.URL.Query().Get("env"); got != "production" {
			t.Errorf("expected env=production, got %s", got)
		}

		var body struct {
			Secrets []Secret `json:"secrets"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if len(body.Secrets) != 1 {
			t.Fatalf("expected 1 secret, got %d", len(body.Secrets))
		}

		if body.Secrets[0].Key != "DATABASE_URL" {
			t.Errorf("expected key DATABASE_URL, got %s", body.Secrets[0].Key)
		}

		if body.Secrets[0].Value != "postgres://localhost" {
			t.Errorf(
				"expected value postgres://localhost, got %s",
				body.Secrets[0].Value,
			)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"id": "secret-123",
				"key": "DATABASE_URL",
				"value": "postgres://localhost",
				"comment": "Database connection"
			}
		]`))
	}))
	defer server.Close()

	c := newTestClient(server)

	secret := Secret{
		Key:   "DATABASE_URL",
		Value: "postgres://localhost",
	}

	got, err := CreateSecret(c, "app-123", "production", secret)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("expected secret, got nil")
	}

	if got.ID != "secret-123" {
		t.Errorf("expected ID secret-123, got %s", got.ID)
	}

	if got.Key != "DATABASE_URL" {
		t.Errorf("expected key DATABASE_URL, got %s", got.Key)
	}

	if got.Value != "postgres://localhost" {
		t.Errorf("expected value postgres://localhost, got %s", got.Value)
	}
}

func TestCreateSecretAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"secret already exists"}`))
	}))
	defer server.Close()

	c := newTestClient(server)

	_, err := CreateSecret(
		c,
		"app-123",
		"production",
		Secret{
			Key:   "DATABASE_URL",
			Value: "test",
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "failed to create secret") {
		t.Errorf("expected wrapped create error, got %v", err)
	}

	var apiErr *client.APIError
	if !errors.As(err, &apiErr) {
		t.Fatal("expected error to contain *client.APIError")
	}

	if apiErr.StatusCode != http.StatusConflict {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusConflict,
			apiErr.StatusCode,
		)
	}

	if string(apiErr.Body) != `{"message":"secret already exists"}` {
		t.Errorf("unexpected response body: %s", apiErr.Body)
	}
}

func TestCreateSecretEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newTestClient(server)

	_, err := CreateSecret(
		c,
		"app-123",
		"production",
		Secret{
			Key:   "DATABASE_URL",
			Value: "test",
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "no secret created" {
		t.Errorf("expected 'no secret created', got %q", err.Error())
	}
}

func TestCreateSecretInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`invalid json`))
	}))
	defer server.Close()

	c := newTestClient(server)

	_, err := CreateSecret(
		c,
		"app-123",
		"production",
		Secret{
			Key:   "DATABASE_URL",
			Value: "test",
		},
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestReadSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		if r.URL.Query().Get("app_id") != "app-123" {
			t.Errorf("unexpected app_id: %s", r.URL.Query().Get("app_id"))
		}

		if r.URL.Query().Get("env") != "production" {
			t.Errorf("unexpected env: %s", r.URL.Query().Get("env"))
		}

		if r.URL.Query().Get("key") != "DATABASE_URL" {
			t.Errorf("unexpected key: %s", r.URL.Query().Get("key"))
		}

		_, _ = w.Write([]byte(`[
			{
				"id": "secret-123",
				"key": "DATABASE_URL",
				"value": "postgres://localhost"
			}
		]`))
	}))
	defer server.Close()

	c := newTestClient(server)

	got, err := ReadSecret(
		c,
		"app-123",
		"production",
		"DATABASE_URL",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(got))
	}

	if got[0].Key != "DATABASE_URL" {
		t.Errorf("expected DATABASE_URL, got %s", got[0].Key)
	}
}

func TestReadSecretWithTags(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tags := r.URL.Query().Get("tags")

		if tags != "production,database" {
			t.Errorf("expected tags=production,database, got %s", tags)
		}

		_, _ = w.Write([]byte(`[
			{
				"id": "secret-123",
				"key": "DATABASE_URL",
				"value": "test",
				"tags": ["production", "database"]
			}
		]`))
	}))
	defer server.Close()

	c := newTestClient(server)

	got, err := ReadSecret(
		c,
		"app-123",
		"production",
		"",
		"production",
		"database",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 secret, got %d", len(got))
	}

	if len(got[0].Tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(got[0].Tags))
	}
}

func TestReadSecretEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[]`))
	}))
	defer server.Close()

	c := newTestClient(server)

	_, err := ReadSecret(
		c,
		"app-123",
		"production",
		"",
	)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if err.Error() != "no secrets found" {
		t.Errorf("expected 'no secrets found', got %q", err.Error())
	}
}

func TestUpdateSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}

		var body struct {
			Secrets []Secret `json:"secrets"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if len(body.Secrets) != 1 {
			t.Fatalf("expected 1 secret, got %d", len(body.Secrets))
		}

		if body.Secrets[0].ID != "secret-123" {
			t.Errorf("expected secret ID secret-123, got %s", body.Secrets[0].ID)
		}

		_, _ = w.Write([]byte(`[
			{
				"id": "secret-123",
				"key": "DATABASE_URL",
				"value": "updated-value"
			}
		]`))
	}))
	defer server.Close()

	c := newTestClient(server)

	got, err := UpdateSecret(
		c,
		"app-123",
		"production",
		Secret{
			ID:    "secret-123",
			Key:   "DATABASE_URL",
			Value: "updated-value",
		},
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got == nil {
		t.Fatal("expected secret, got nil")
	}

	if got.Value != "updated-value" {
		t.Errorf("expected updated-value, got %s", got.Value)
	}
}

func TestDeleteSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		var body struct {
			Secrets []string `json:"secrets"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if len(body.Secrets) != 1 {
			t.Fatalf("expected 1 secret ID, got %d", len(body.Secrets))
		}

		if body.Secrets[0] != "secret-123" {
			t.Errorf("expected secret-123, got %s", body.Secrets[0])
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := newTestClient(server)

	err := DeleteSecret(
		c,
		"app-123",
		"production",
		"secret-123",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListSecrets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}

		if r.URL.Query().Get("path") != "/database" {
			t.Errorf("expected path=/database, got %s", r.URL.Query().Get("path"))
		}

		_, _ = w.Write([]byte(`[
			{
				"id": "secret-123",
				"key": "DATABASE_URL",
				"value": "postgres://localhost",
				"path": "/database"
			},
			{
				"id": "secret-456",
				"key": "DATABASE_USER",
				"value": "postgres",
				"path": "/database"
			}
		]`))
	}))
	defer server.Close()

	c := newTestClient(server)

	got, err := ListSecrets(
		c,
		"app-123",
		"production",
		"/database",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("expected 2 secrets, got %d", len(got))
	}

	if got[0].Key != "DATABASE_URL" {
		t.Errorf("expected DATABASE_URL, got %s", got[0].Key)
	}

	if got[1].Key != "DATABASE_USER" {
		t.Errorf("expected DATABASE_USER, got %s", got[1].Key)
	}
}
