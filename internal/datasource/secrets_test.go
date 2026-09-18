package datasource

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/terraform-provider/internal/client"
	"github.com/phasehq/terraform-provider/internal/secrets"
)

func newTestSecretsDataSourceClient(
	t *testing.T,
	handler http.Handler,
) *client.PhaseClient {
	t.Helper()

	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return &client.PhaseClient{
		HostURL:    server.URL,
		HTTPClient: server.Client(),
		Token:      "test-token",
		TokenType:  "User",
	}
}

func writeSecretsDataSourceResponse(
	t *testing.T,
	w http.ResponseWriter,
	secretList []secrets.Secret,
) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(secretList); err != nil {
		t.Fatalf("failed to encode response: %v", err)
	}
}

func writeSecretsDataSourceError(
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}

func TestSecretsSchema(t *testing.T) {
	resource := Secrets()

	tests := []struct {
		name      string
		fieldType schema.ValueType
		required  bool
		optional  bool
		computed  bool
		sensitive bool
	}{
		{
			name:      "app_id",
			fieldType: schema.TypeString,
			required:  true,
		},
		{
			name:      "env",
			fieldType: schema.TypeString,
			required:  true,
		},
		{
			name:      "path",
			fieldType: schema.TypeString,
			optional:  true,
		},
		{
			name:      "key",
			fieldType: schema.TypeString,
			optional:  true,
		},
		{
			name:      "tags",
			fieldType: schema.TypeList,
			optional:  true,
		},
		{
			name:      "secrets",
			fieldType: schema.TypeMap,
			computed:  true,
			sensitive: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := resource.Schema[tt.name]
			if !ok {
				t.Fatalf(
					"field %q not found in schema",
					tt.name,
				)
			}

			if field.Type != tt.fieldType {
				t.Errorf(
					"expected type %v, got %v",
					tt.fieldType,
					field.Type,
				)
			}

			if field.Required != tt.required {
				t.Errorf(
					"expected Required=%v, got %v",
					tt.required,
					field.Required,
				)
			}

			if field.Optional != tt.optional {
				t.Errorf(
					"expected Optional=%v, got %v",
					tt.optional,
					field.Optional,
				)
			}

			if field.Computed != tt.computed {
				t.Errorf(
					"expected Computed=%v, got %v",
					tt.computed,
					field.Computed,
				)
			}

			if field.Sensitive != tt.sensitive {
				t.Errorf(
					"expected Sensitive=%v, got %v",
					tt.sensitive,
					field.Sensitive,
				)
			}
		})
	}

	if resource.Schema["tags"].Elem == nil {
		t.Fatal("expected tags Elem to be configured")
	}
}

func TestDataSourceSecretsRead(t *testing.T) {
	const (
		appID = "app-123"
		env   = "production"
		path  = "/database/"
		key   = "DATABASE_URL"
	)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf(
				"expected GET, got %s",
				r.Method,
			)
			return
		}

		if r.URL.Path != "/v1/secrets/" {
			t.Errorf(
				"expected path /v1/secrets/, got %s",
				r.URL.Path,
			)
		}

		if got := r.URL.Query().Get("app_id"); got != appID {
			t.Errorf(
				"expected app_id %q, got %q",
				appID,
				got,
			)
		}

		if got := r.URL.Query().Get("env"); got != env {
			t.Errorf(
				"expected env %q, got %q",
				env,
				got,
			)
		}

		if got := r.URL.Query().Get("key"); got != key {
			t.Errorf(
				"expected key %q, got %q",
				key,
				got,
			)
		}

		if got := r.URL.Query().Get("tags"); got != "database,production" {
			t.Errorf(
				"expected tags database,production, got %q",
				got,
			)
		}

		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:      "secret-1",
				Key:     "DATABASE_URL",
				Value:   "postgres://localhost",
				Path:    "/database/",
				Tags:    []string{"database", "production"},
				Version: 1,
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": appID,
			"env":    env,
			"path":   path,
			"key":    key,
			"tags": []any{
				"database",
				"production",
			},
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	expected := map[string]string{
		"DATABASE_URL": "postgres://localhost",
	}

	gotRaw := d.Get("secrets").(map[string]any)

	got := make(map[string]string)

	for key, value := range gotRaw {
		got[key] = value.(string)
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf(
			"expected secrets %v, got %v",
			expected,
			got,
		)
	}

	expectedID := "app-123-production-/database/-DATABASE_URL-database,production"

	if got := d.Id(); got != expectedID {
		t.Errorf(
			"expected ID %q, got %q",
			expectedID,
			got,
		)
	}

	if got := d.Get("path"); got != path {
		t.Errorf(
			"expected path %q, got %v",
			path,
			got,
		)
	}
}

func TestDataSourceSecretsReadWithoutKey(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("key"); got != "" {
			t.Errorf(
				"expected no key query parameter, got %q",
				got,
			)
		}

		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "DATABASE_URL",
				Value: "postgres://localhost",
				Path:  "/",
			},
			{
				ID:    "secret-2",
				Key:   "API_KEY",
				Value: "normal-api-key",
				Path:  "/",
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	gotRaw := d.Get("secrets").(map[string]any)

	expected := map[string]any{
		"DATABASE_URL": "postgres://localhost",
		"API_KEY":      "normal-api-key",
	}

	if !reflect.DeepEqual(gotRaw, expected) {
		t.Errorf(
			"expected secrets %v, got %v",
			expected,
			gotRaw,
		)
	}
}

func TestDataSourceSecretsReadWithoutTags(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, exists := r.URL.Query()["tags"]; exists {
			t.Error("expected tags query parameter to be absent")
		}

		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "API_KEY",
				Value: "api-value",
				Path:  "/",
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	got := d.Get("secrets").(map[string]any)

	if got["API_KEY"] != "api-value" {
		t.Errorf(
			"expected API_KEY=api-value, got %v",
			got["API_KEY"],
		)
	}
}

func TestDataSourceSecretsReadWithOverride(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "API_KEY",
				Value: "normal-value",
				Path:  "/",
				Override: &secrets.SecretOverride{
					Value:    "override-value",
					IsActive: true,
				},
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	got := d.Get("secrets").(map[string]any)

	if got["API_KEY"] != "override-value" {
		t.Errorf(
			"expected override-value, got %v",
			got["API_KEY"],
		)
	}
}

func TestDataSourceSecretsReadWithInactiveOverride(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "API_KEY",
				Value: "normal-value",
				Path:  "/",
				Override: &secrets.SecretOverride{
					Value:    "override-value",
					IsActive: false,
				},
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	got := d.Get("secrets").(map[string]any)

	if got["API_KEY"] != "normal-value" {
		t.Errorf(
			"expected normal-value, got %v",
			got["API_KEY"],
		)
	}
}

func TestDataSourceSecretsReadFiltersByPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "DATABASE_URL",
				Value: "database-value",
				Path:  "/database/",
			},
			{
				ID:    "secret-2",
				Key:   "API_KEY",
				Value: "api-value",
				Path:  "/api/",
			},
			{
				ID:    "secret-3",
				Key:   "OTHER",
				Value: "other-value",
				Path:  "/",
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/database/",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	got := d.Get("secrets").(map[string]any)

	expected := map[string]any{
		"DATABASE_URL": "database-value",
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf(
			"expected %v, got %v",
			expected,
			got,
		)
	}
}

func TestDataSourceSecretsReadWithEmptyPath(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "DATABASE_URL",
				Value: "database-value",
				Path:  "/database/",
			},
			{
				ID:    "secret-2",
				Key:   "API_KEY",
				Value: "api-value",
				Path:  "/api/",
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	got := d.Get("secrets").(map[string]any)

	expected := map[string]any{
		"DATABASE_URL": "database-value",
		"API_KEY":      "api-value",
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf(
			"expected %v, got %v",
			expected,
			got,
		)
	}

	if got := d.Get("path"); got != "" {
		t.Errorf(
			"expected path to remain empty, got %v",
			got,
		)
	}
}

func TestDataSourceSecretsReadWithMultipleTags(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("tags"); got != "backend,production,public" {
			t.Errorf(
				"expected tags backend,production,public, got %q",
				got,
			)
		}

		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "API_URL",
				Value: "https://api.example.com",
				Path:  "/",
				Tags: []string{
					"backend",
					"production",
				},
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
			"tags": []any{
				"backend",
				"production",
				"public",
			},
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	got := d.Get("secrets").(map[string]any)

	if got["API_URL"] != "https://api.example.com" {
		t.Errorf(
			"expected API_URL value, got %v",
			got["API_URL"],
		)
	}
}

func TestDataSourceSecretsReadReturnsAPIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceError(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}

	if !strings.Contains(
		diags[0].Summary,
		"failed to read secret",
	) {
		t.Errorf(
			"expected failed to read secret error, got %q",
			diags[0].Summary,
		)
	}
}

func TestDataSourceSecretsReadWithoutSecrets(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceResponse(
			t,
			w,
			[]secrets.Secret{},
		)
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}

	if !strings.Contains(
		diags[0].Summary,
		"no secrets found",
	) {
		t.Errorf(
			"expected no secrets found error, got %q",
			diags[0].Summary,
		)
	}
}

func TestDataSourceSecretsReadWithKeyAndPathMismatch(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "DATABASE_URL",
				Value: "database-value",
				Path:  "/other/",
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/database/",
			"key":    "DATABASE_URL",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	got := d.Get("secrets").(map[string]any)

	if len(got) != 0 {
		t.Errorf(
			"expected empty secrets map, got %v",
			got,
		)
	}
}

func TestDataSourceSecretsReadGeneratesExpectedID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsDataSourceResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-1",
				Key:   "API_KEY",
				Value: "api-value",
				Path:  "/",
			},
		})
	})

	phaseClient := newTestSecretsDataSourceClient(
		t,
		handler,
	)

	d := schema.TestResourceDataRaw(
		t,
		Secrets().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"path":   "/",
			"key":    "API_KEY",
		},
	)

	diags := dataSourceSecretsRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf(
			"unexpected diagnostics: %v",
			diags,
		)
	}

	expectedID := "app-123-production-/-API_KEY-"

	if got := d.Id(); got != expectedID {
		t.Errorf(
			"expected ID %q, got %q",
			expectedID,
			got,
		)
	}
}
