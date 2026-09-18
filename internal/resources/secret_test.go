package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/terraform-provider/internal/client"
	"github.com/phasehq/terraform-provider/internal/secrets"
)

func newTestSecretClient(t *testing.T, handler http.Handler) *client.PhaseClient {
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

func decodeSecretRequest(t *testing.T, r *http.Request) map[string]any {
	t.Helper()

	var body struct {
		Secrets []map[string]any `json:"secrets"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode request body: %v", err)
	}

	if len(body.Secrets) != 1 {
		t.Fatalf("expected one secret, got %d", len(body.Secrets))
	}

	return body.Secrets[0]
}

func writeSecretsResponse(
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

func writeErrorResponse(
	w http.ResponseWriter,
	status int,
	message string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(message))
}

func TestSecretSchema(t *testing.T) {
	resource := Secret()

	tests := []struct {
		field     string
		fieldType schema.ValueType
		required  bool
		optional  bool
		computed  bool
		forceNew  bool
		sensitive bool
	}{
		{
			field:     "app_id",
			fieldType: schema.TypeString,
			required:  true,
			forceNew:  true,
		},
		{
			field:     "env",
			fieldType: schema.TypeString,
			required:  true,
			forceNew:  true,
		},
		{
			field:     "key",
			fieldType: schema.TypeString,
			required:  true,
		},
		{
			field:     "value",
			fieldType: schema.TypeString,
			required:  true,
			sensitive: true,
		},
		{
			field:     "comment",
			fieldType: schema.TypeString,
			optional:  true,
		},
		{
			field:     "path",
			fieldType: schema.TypeString,
			optional:  true,
		},
		{
			field:     "tags",
			fieldType: schema.TypeList,
			optional:  true,
		},
		{
			field:     "version",
			fieldType: schema.TypeInt,
			computed:  true,
		},
		{
			field:     "created_at",
			fieldType: schema.TypeString,
			computed:  true,
		},
		{
			field:     "updated_at",
			fieldType: schema.TypeString,
			computed:  true,
		},
		{
			field:     "override",
			fieldType: schema.TypeSet,
			optional:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			field, ok := resource.Schema[tt.field]
			if !ok {
				t.Fatalf("field %q not found in schema", tt.field)
			}

			if field.Type != tt.fieldType {
				t.Errorf(
					"expected field %q to have type %v, got %v",
					tt.field,
					tt.fieldType,
					field.Type,
				)
			}

			if field.Required != tt.required {
				t.Errorf(
					"expected field %q Required=%v, got %v",
					tt.field,
					tt.required,
					field.Required,
				)
			}

			if field.Optional != tt.optional {
				t.Errorf(
					"expected field %q Optional=%v, got %v",
					tt.field,
					tt.optional,
					field.Optional,
				)
			}

			if field.Computed != tt.computed {
				t.Errorf(
					"expected field %q Computed=%v, got %v",
					tt.field,
					tt.computed,
					field.Computed,
				)
			}

			if field.ForceNew != tt.forceNew {
				t.Errorf(
					"expected field %q ForceNew=%v, got %v",
					tt.field,
					tt.forceNew,
					field.ForceNew,
				)
			}

			if field.Sensitive != tt.sensitive {
				t.Errorf(
					"expected field %q Sensitive=%v, got %v",
					tt.field,
					tt.sensitive,
					field.Sensitive,
				)
			}
		})
	}

	if got := resource.Schema["path"].Default; got != "/" {
		t.Fatalf("expected path default '/', got %v", got)
	}

	if resource.Schema["override"].MaxItems != 1 {
		t.Fatalf(
			"expected override MaxItems=1, got %d",
			resource.Schema["override"].MaxItems,
		)
	}
}

func TestResourceSecretCreateSuccess(t *testing.T) {
	const (
		appID = "app-123"
		env   = "production"
	)

	var postCalled bool
	var getCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCalled = true

			if r.URL.Path != "/v1/secrets/" {
				t.Errorf("expected path /v1/secrets/, got %s", r.URL.Path)
			}

			if r.URL.Query().Get("app_id") != appID {
				t.Errorf(
					"expected app_id %q, got %q",
					appID,
					r.URL.Query().Get("app_id"),
				)
			}

			if r.URL.Query().Get("env") != env {
				t.Errorf(
					"expected env %q, got %q",
					env,
					r.URL.Query().Get("env"),
				)
			}

			secret := decodeSecretRequest(t, r)

			if secret["key"] != "DATABASE_URL" {
				t.Errorf(
					"expected key DATABASE_URL, got %v",
					secret["key"],
				)
			}

			if secret["value"] != "postgres://localhost" {
				t.Errorf(
					"expected value postgres://localhost, got %v",
					secret["value"],
				)
			}

			if secret["comment"] != "database connection" {
				t.Errorf(
					"expected comment database connection, got %v",
					secret["comment"],
				)
			}

			if secret["path"] != "/database/" {
				t.Errorf(
					"expected path /database/, got %v",
					secret["path"],
				)
			}

			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:        "secret-123",
					Key:       "DATABASE_URL",
					Value:     "postgres://localhost",
					Comment:   "database connection",
					Path:      "/database/",
					Version:   1,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T10:00:00Z",
				},
			})

		case http.MethodGet:
			getCalled = true

			if r.URL.Query().Get("key") != "DATABASE_URL" {
				t.Errorf(
					"expected key DATABASE_URL, got %q",
					r.URL.Query().Get("key"),
				)
			}

			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:        "secret-123",
					Key:       "DATABASE_URL",
					Value:     "postgres://localhost",
					Comment:   "database connection",
					Path:      "/database/",
					Version:   1,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T10:00:00Z",
				},
			})

		default:
			t.Errorf("unexpected method %s", r.Method)
			writeErrorResponse(
				w,
				http.StatusMethodNotAllowed,
				"method not allowed",
			)
		}
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id":  appID,
			"env":     env,
			"key":     "DATABASE_URL",
			"value":   "postgres://localhost",
			"comment": "database connection",
			"path":    "/database/",
		},
	)

	diags := resourceSecretCreate(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !postCalled {
		t.Fatal("expected POST request")
	}

	if !getCalled {
		t.Fatal("expected GET request after create")
	}

	if got := d.Id(); got != "secret-123" {
		t.Errorf("expected ID secret-123, got %q", got)
	}

	if got := d.Get("key"); got != "DATABASE_URL" {
		t.Errorf("expected key DATABASE_URL, got %v", got)
	}

	if got := d.Get("value"); got != "postgres://localhost" {
		t.Errorf(
			"expected value postgres://localhost, got %v",
			got,
		)
	}

	if got := d.Get("comment"); got != "database connection" {
		t.Errorf(
			"expected comment database connection, got %v",
			got,
		)
	}

	if got := d.Get("path"); got != "/database/" {
		t.Errorf(
			"expected path /database/, got %v",
			got,
		)
	}

	if got := d.Get("version"); got != 1 {
		t.Errorf("expected version 1, got %v", got)
	}

	if got := d.Get("created_at"); got != "2026-09-18T10:00:00Z" {
		t.Errorf(
			"expected created_at 2026-09-18T10:00:00Z, got %v",
			got,
		)
	}

	if got := d.Get("updated_at"); got != "2026-09-18T10:00:00Z" {
		t.Errorf(
			"expected updated_at 2026-09-18T10:00:00Z, got %v",
			got,
		)
	}
}

func TestResourceSecretCreateWithTags(t *testing.T) {
	var postCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCalled = true

			secret := decodeSecretRequest(t, r)

			rawTags, ok := secret["tags"]
			if !ok {
				t.Fatal("expected tags in request")
			}

			tags, ok := rawTags.([]any)
			if !ok {
				t.Fatalf(
					"expected tags array, got %T",
					rawTags,
				)
			}

			expectedTags := []string{
				"database",
				"production",
			}

			if len(tags) != len(expectedTags) {
				t.Fatalf(
					"expected %d tags, got %d",
					len(expectedTags),
					len(tags),
				)
			}

			for i, expected := range expectedTags {
				if tags[i] != expected {
					t.Errorf(
						"expected tag %d to be %q, got %v",
						i,
						expected,
						tags[i],
					)
				}
			}

			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:        "secret-tags",
					Key:       "DATABASE_URL",
					Value:     "postgres://localhost",
					Tags:      expectedTags,
					Version:   1,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T10:00:00Z",
				},
			})

		case http.MethodGet:
			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:    "secret-tags",
					Key:   "DATABASE_URL",
					Value: "postgres://localhost",
					Tags: []string{
						"database",
						"production",
					},
					Version:   1,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T10:00:00Z",
				},
			})

		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "postgres://localhost",
			"tags": []any{
				"database",
				"production",
			},
		},
	)

	diags := resourceSecretCreate(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !postCalled {
		t.Fatal("expected POST request")
	}

	if got := d.Id(); got != "secret-tags" {
		t.Errorf(
			"expected ID secret-tags, got %q",
			got,
		)
	}

	tags := d.Get("tags").([]any)

	expectedTags := []any{
		"database",
		"production",
	}

	if !reflect.DeepEqual(tags, expectedTags) {
		t.Errorf(
			"expected tags %v, got %v",
			expectedTags,
			tags,
		)
	}
}

func TestResourceSecretCreateWithOverride(t *testing.T) {
	var postCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCalled = true

			secret := decodeSecretRequest(t, r)

			if secret["key"] != "API_KEY" {
				t.Errorf(
					"expected key API_KEY, got %v",
					secret["key"],
				)
			}

			if secret["value"] != "normal-value" {
				t.Errorf(
					"expected value normal-value, got %v",
					secret["value"],
				)
			}

			override, ok := secret["override"].(map[string]any)
			if !ok {
				t.Fatalf(
					"expected override object, got %T (%v)",
					secret["override"],
					secret["override"],
				)
			}

			if override["value"] != "override-value" {
				t.Errorf(
					"expected override value override-value, got %v",
					override["value"],
				)
			}

			// SecretOverride uses json:"isActive".
			if override["isActive"] != true {
				t.Errorf(
					"expected override isActive true, got %v",
					override["isActive"],
				)
			}

			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:    "secret-override",
					Key:   "API_KEY",
					Value: "normal-value",
					Override: &secrets.SecretOverride{
						Value:    "override-value",
						IsActive: true,
					},
					Version:   1,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T10:00:00Z",
				},
			})

		case http.MethodGet:
			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:    "secret-override",
					Key:   "API_KEY",
					Value: "normal-value",
					Override: &secrets.SecretOverride{
						Value:    "override-value",
						IsActive: true,
					},
					Version:   1,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T10:00:00Z",
				},
			})

		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "API_KEY",
			"value":  "normal-value",
			"override": []any{
				map[string]any{
					"value":     "override-value",
					"is_active": true,
				},
			},
		},
	)

	diags := resourceSecretCreate(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !postCalled {
		t.Fatal("expected POST request")
	}

	if got := d.Id(); got != "secret-override" {
		t.Errorf(
			"expected ID secret-override, got %q",
			got,
		)
	}

	if got := d.Get("value"); got != "override-value" {
		t.Errorf(
			"expected value override-value, got %v",
			got,
		)
	}

	overrideSet := d.Get("override").(*schema.Set)

	if overrideSet.Len() != 1 {
		t.Fatalf(
			"expected one override item, got %d",
			overrideSet.Len(),
		)
	}

	override := overrideSet.List()[0].(map[string]any)

	if override["value"] != "override-value" {
		t.Errorf(
			"expected override value override-value, got %v",
			override["value"],
		)
	}

	if override["is_active"] != true {
		t.Errorf(
			"expected override is_active true, got %v",
			override["is_active"],
		)
	}
}

func TestResourceSecretCreateConflictUpdatesExistingSecret(t *testing.T) {
	var postCalled bool
	var getCalled int
	var putCalled bool

	currentSecret := secrets.Secret{
		ID:        "existing-secret",
		Key:       "DATABASE_URL",
		Value:     "old-value",
		Version:   3,
		CreatedAt: "2026-09-18T10:00:00Z",
		UpdatedAt: "2026-09-18T11:00:00Z",
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			postCalled = true

			if r.URL.Path != "/v1/secrets/" {
				t.Errorf(
					"expected path /v1/secrets/, got %s",
					r.URL.Path,
				)
			}

			decodeSecretRequest(t, r)

			writeErrorResponse(
				w,
				http.StatusConflict,
				"409 Conflict",
			)

		case http.MethodGet:
			getCalled++

			if r.URL.Query().Get("key") != "DATABASE_URL" {
				t.Errorf(
					"expected key DATABASE_URL, got %q",
					r.URL.Query().Get("key"),
				)
			}

			writeSecretsResponse(t, w, []secrets.Secret{
				currentSecret,
			})

		case http.MethodPut:
			putCalled = true

			secret := decodeSecretRequest(t, r)

			if secret["id"] != "existing-secret" {
				t.Errorf(
					"expected existing secret ID, got %v",
					secret["id"],
				)
			}

			if secret["value"] != "new-value" {
				t.Errorf(
					"expected new-value, got %v",
					secret["value"],
				)
			}

			// Simulate the API updating the existing secret.
			currentSecret.Value = "new-value"
			currentSecret.Version = 4
			currentSecret.UpdatedAt = "2026-09-18T12:00:00Z"

			writeSecretsResponse(t, w, []secrets.Secret{
				currentSecret,
			})

		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "new-value",
		},
	)

	diags := resourceSecretCreate(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !postCalled {
		t.Fatal("expected POST request")
	}

	if getCalled != 2 {
		t.Fatalf(
			"expected 2 GET requests, got %d",
			getCalled,
		)
	}

	if !putCalled {
		t.Fatal("expected PUT request")
	}

	if got := d.Id(); got != "existing-secret" {
		t.Errorf(
			"expected existing secret ID, got %q",
			got,
		)
	}

	if got := d.Get("value"); got != "new-value" {
		t.Errorf(
			"expected new-value, got %v",
			got,
		)
	}

	if got := d.Get("version"); got != 4 {
		t.Errorf(
			"expected version 4, got %v",
			got,
		)
	}

	if got := d.Get("updated_at"); got != "2026-09-18T12:00:00Z" {
		t.Errorf(
			"expected updated_at 2026-09-18T12:00:00Z, got %v",
			got,
		)
	}
}

func TestResourceSecretCreateReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf(
				"expected POST, got %s",
				r.Method,
			)
			return
		}

		writeErrorResponse(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "postgres://localhost",
		},
	)

	diags := resourceSecretCreate(
		context.Background(),
		d,
		phaseClient,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}
}

func TestResourceSecretRead(t *testing.T) {
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

		if r.URL.Query().Get("app_id") != "app-123" {
			t.Errorf(
				"expected app_id app-123, got %q",
				r.URL.Query().Get("app_id"),
			)
		}

		if r.URL.Query().Get("env") != "production" {
			t.Errorf(
				"expected env production, got %q",
				r.URL.Query().Get("env"),
			)
		}

		if r.URL.Query().Get("key") != "DATABASE_URL" {
			t.Errorf(
				"expected key DATABASE_URL, got %q",
				r.URL.Query().Get("key"),
			)
		}

		writeSecretsResponse(t, w, []secrets.Secret{
			{
				ID:        "secret-123",
				Key:       "DATABASE_URL",
				Value:     "postgres://localhost",
				Comment:   "database",
				Path:      "/",
				Tags:      []string{"database"},
				Version:   2,
				CreatedAt: "2026-09-18T10:00:00Z",
				UpdatedAt: "2026-09-18T11:00:00Z",
			},
		})
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "old-value",
		},
	)

	d.SetId("old-id")

	diags := resourceSecretRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := d.Id(); got != "secret-123" {
		t.Errorf(
			"expected ID secret-123, got %q",
			got,
		)
	}

	if got := d.Get("key"); got != "DATABASE_URL" {
		t.Errorf(
			"expected key DATABASE_URL, got %v",
			got,
		)
	}

	if got := d.Get("value"); got != "postgres://localhost" {
		t.Errorf(
			"expected value postgres://localhost, got %v",
			got,
		)
	}

	if got := d.Get("comment"); got != "database" {
		t.Errorf(
			"expected comment database, got %v",
			got,
		)
	}

	if got := d.Get("path"); got != "/" {
		t.Errorf(
			"expected path /, got %v",
			got,
		)
	}

	tags := d.Get("tags").([]any)

	if !reflect.DeepEqual(
		tags,
		[]any{"database"},
	) {
		t.Errorf(
			"expected tags [database], got %v",
			tags,
		)
	}

	if got := d.Get("version"); got != 2 {
		t.Errorf(
			"expected version 2, got %v",
			got,
		)
	}
}

func TestResourceSecretReadWithOverride(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf(
				"expected GET, got %s",
				r.Method,
			)
			return
		}

		writeSecretsResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-override",
				Key:   "API_KEY",
				Value: "normal-value",
				Override: &secrets.SecretOverride{
					Value:    "override-value",
					IsActive: true,
				},
				Version:   2,
				CreatedAt: "2026-09-18T10:00:00Z",
				UpdatedAt: "2026-09-18T11:00:00Z",
			},
		})
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "API_KEY",
			"value":  "normal-value",
		},
	)

	d.SetId("secret-override")

	diags := resourceSecretRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := d.Get("value"); got != "override-value" {
		t.Errorf(
			"expected override-value, got %v",
			got,
		)
	}

	overrideSet := d.Get("override").(*schema.Set)

	if overrideSet.Len() != 1 {
		t.Fatalf(
			"expected one override item, got %d",
			overrideSet.Len(),
		)
	}

	override := overrideSet.List()[0].(map[string]any)

	if override["value"] != "override-value" {
		t.Errorf(
			"expected override-value, got %v",
			override["value"],
		)
	}

	if override["is_active"] != true {
		t.Errorf(
			"expected is_active true, got %v",
			override["is_active"],
		)
	}
}

func TestResourceSecretReadWithInactiveOverride(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-123",
				Key:   "API_KEY",
				Value: "normal-value",
				Override: &secrets.SecretOverride{
					Value:    "override-value",
					IsActive: false,
				},
			},
		})
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "API_KEY",
			"value":  "old-value",
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretRead(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if got := d.Get("value"); got != "normal-value" {
		t.Errorf(
			"expected normal-value, got %v",
			got,
		)
	}

	overrideSet := d.Get("override").(*schema.Set)

	if overrideSet.Len() != 0 {
		t.Errorf(
			"expected no override items, got %d",
			overrideSet.Len(),
		)
	}
}

func TestResourceSecretReadWithoutSecrets(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsResponse(
			t,
			w,
			[]secrets.Secret{},
		)
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "old-value",
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretRead(
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
			"expected 'no secrets found', got %q",
			diags[0].Summary,
		)
	}
}

func TestResourceSecretReadReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeErrorResponse(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "old-value",
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretRead(
		context.Background(),
		d,
		phaseClient,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}
}

func TestResourceSecretUpdate(t *testing.T) {
	var putCalled bool
	var getCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			putCalled = true

			if r.URL.Path != "/v1/secrets/" {
				t.Errorf(
					"expected path /v1/secrets/, got %s",
					r.URL.Path,
				)
			}

			secret := decodeSecretRequest(t, r)

			if secret["id"] != "secret-123" {
				t.Errorf(
					"expected ID secret-123, got %v",
					secret["id"],
				)
			}

			if secret["key"] != "DATABASE_URL" {
				t.Errorf(
					"expected key DATABASE_URL, got %v",
					secret["key"],
				)
			}

			if secret["value"] != "new-value" {
				t.Errorf(
					"expected value new-value, got %v",
					secret["value"],
				)
			}

			if secret["comment"] != "updated comment" {
				t.Errorf(
					"expected comment updated comment, got %v",
					secret["comment"],
				)
			}

			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:        "secret-123",
					Key:       "DATABASE_URL",
					Value:     "new-value",
					Comment:   "updated comment",
					Version:   2,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T12:00:00Z",
				},
			})

		case http.MethodGet:
			getCalled = true

			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:        "secret-123",
					Key:       "DATABASE_URL",
					Value:     "new-value",
					Comment:   "updated comment",
					Version:   2,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T12:00:00Z",
				},
			})

		default:
			t.Errorf(
				"unexpected method %s",
				r.Method,
			)
		}
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id":  "app-123",
			"env":     "production",
			"key":     "DATABASE_URL",
			"value":   "new-value",
			"comment": "updated comment",
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretUpdate(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !putCalled {
		t.Fatal("expected PUT request")
	}

	if !getCalled {
		t.Fatal("expected GET request after update")
	}

	if got := d.Id(); got != "secret-123" {
		t.Errorf(
			"expected ID secret-123, got %q",
			got,
		)
	}

	if got := d.Get("key"); got != "DATABASE_URL" {
		t.Errorf(
			"expected key DATABASE_URL, got %v",
			got,
		)
	}

	if got := d.Get("value"); got != "new-value" {
		t.Errorf(
			"expected value new-value, got %v",
			got,
		)
	}

	if got := d.Get("comment"); got != "updated comment" {
		t.Errorf(
			"expected comment updated comment, got %v",
			got,
		)
	}
}

func TestResourceSecretUpdateWithTagsAndOverride(t *testing.T) {
	var putCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			putCalled = true

			secret := decodeSecretRequest(t, r)

			rawTags, ok := secret["tags"]
			if !ok {
				t.Fatal("expected tags in request")
			}

			tags, ok := rawTags.([]any)
			if !ok {
				t.Fatalf(
					"expected tags array, got %T",
					rawTags,
				)
			}

			if !reflect.DeepEqual(
				tags,
				[]any{
					"api",
					"production",
				},
			) {
				t.Errorf(
					"expected tags [api production], got %v",
					tags,
				)
			}

			override, ok := secret["override"].(map[string]any)
			if !ok {
				t.Fatalf(
					"expected override object, got %T",
					secret["override"],
				)
			}

			if override["value"] != "override-value" {
				t.Errorf(
					"expected override-value, got %v",
					override["value"],
				)
			}

			if override["isActive"] != true {
				t.Errorf(
					"expected isActive true, got %v",
					override["isActive"],
				)
			}

			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:    "secret-123",
					Key:   "API_KEY",
					Value: "normal-value",
					Tags: []string{
						"api",
						"production",
					},
					Override: &secrets.SecretOverride{
						Value:    "override-value",
						IsActive: true,
					},
					Version:   2,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T12:00:00Z",
				},
			})

		case http.MethodGet:
			writeSecretsResponse(t, w, []secrets.Secret{
				{
					ID:    "secret-123",
					Key:   "API_KEY",
					Value: "normal-value",
					Tags: []string{
						"api",
						"production",
					},
					Override: &secrets.SecretOverride{
						Value:    "override-value",
						IsActive: true,
					},
					Version:   2,
					CreatedAt: "2026-09-18T10:00:00Z",
					UpdatedAt: "2026-09-18T12:00:00Z",
				},
			})

		default:
			t.Errorf(
				"unexpected method %s",
				r.Method,
			)
		}
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "API_KEY",
			"value":  "normal-value",
			"tags": []any{
				"api",
				"production",
			},
			"override": []any{
				map[string]any{
					"value":     "override-value",
					"is_active": true,
				},
			},
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretUpdate(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !putCalled {
		t.Fatal("expected PUT request")
	}

	tags := d.Get("tags").([]any)

	if !reflect.DeepEqual(
		tags,
		[]any{
			"api",
			"production",
		},
	) {
		t.Errorf(
			"expected tags [api production], got %v",
			tags,
		)
	}

	if got := d.Get("value"); got != "override-value" {
		t.Errorf(
			"expected override-value, got %v",
			got,
		)
	}

	overrideSet := d.Get("override").(*schema.Set)

	if overrideSet.Len() != 1 {
		t.Fatalf(
			"expected one override item, got %d",
			overrideSet.Len(),
		)
	}
}

func TestResourceSecretUpdateReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf(
				"expected PUT, got %s",
				r.Method,
			)
			return
		}

		writeErrorResponse(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "new-value",
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretUpdate(
		context.Background(),
		d,
		phaseClient,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}
}

func TestResourceSecretDelete(t *testing.T) {
	var deleteCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf(
				"expected DELETE, got %s",
				r.Method,
			)
			return
		}

		deleteCalled = true

		if r.URL.Path != "/v1/secrets/" {
			t.Errorf(
				"expected /v1/secrets/, got %s",
				r.URL.Path,
			)
		}

		if r.URL.Query().Get("app_id") != "app-123" {
			t.Errorf(
				"expected app_id app-123, got %q",
				r.URL.Query().Get("app_id"),
			)
		}

		if r.URL.Query().Get("env") != "production" {
			t.Errorf(
				"expected env production, got %q",
				r.URL.Query().Get("env"),
			)
		}

		var body struct {
			Secrets []string `json:"secrets"`
		}

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf(
				"failed to decode delete body: %v",
				err,
			)
		}

		if !reflect.DeepEqual(
			body.Secrets,
			[]string{"secret-123"},
		) {
			t.Errorf(
				"expected secrets [secret-123], got %v",
				body.Secrets,
			)
		}

		w.WriteHeader(http.StatusOK)
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "postgres://localhost",
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretDelete(
		context.Background(),
		d,
		phaseClient,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if !deleteCalled {
		t.Fatal("expected DELETE request")
	}

	if got := d.Id(); got != "" {
		t.Errorf(
			"expected empty ID after delete, got %q",
			got,
		)
	}
}

func TestResourceSecretDeleteReturnsError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeErrorResponse(
			w,
			http.StatusInternalServerError,
			"internal server error",
		)
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		map[string]any{
			"app_id": "app-123",
			"env":    "production",
			"key":    "DATABASE_URL",
			"value":  "postgres://localhost",
		},
	)

	d.SetId("secret-123")

	diags := resourceSecretDelete(
		context.Background(),
		d,
		phaseClient,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error")
	}

	if got := d.Id(); got != "secret-123" {
		t.Errorf(
			"expected ID to remain secret-123, got %q",
			got,
		)
	}
}

func TestResourceSecretImportState(t *testing.T) {
	var getCalled bool

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf(
				"expected GET, got %s",
				r.Method,
			)
			return
		}

		getCalled = true

		if r.URL.Path != "/v1/secrets/" {
			t.Errorf(
				"expected /v1/secrets/, got %s",
				r.URL.Path,
			)
		}

		if r.URL.Query().Get("app_id") != "app-123" {
			t.Errorf(
				"expected app_id app-123, got %q",
				r.URL.Query().Get("app_id"),
			)
		}

		if r.URL.Query().Get("env") != "production" {
			t.Errorf(
				"expected env production, got %q",
				r.URL.Query().Get("env"),
			)
		}

		if r.URL.Query().Get("path") != "/database/" {
			t.Errorf(
				"expected path /database/, got %q",
				r.URL.Query().Get("path"),
			)
		}

		writeSecretsResponse(t, w, []secrets.Secret{
			{
				ID:        "secret-123",
				Key:       "DATABASE_URL",
				Value:     "postgres://localhost",
				Comment:   "database connection",
				Path:      "/database/",
				Tags:      []string{"database"},
				Version:   5,
				CreatedAt: "2026-09-18T10:00:00Z",
				UpdatedAt: "2026-09-18T11:00:00Z",
			},
		})
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		nil,
	)

	d.SetId(
		"app-123:production:/database/:DATABASE_URL",
	)

	state, err := resourceSecretImportState(
		context.Background(),
		d,
		phaseClient,
	)

	if err != nil {
		t.Fatalf(
			"unexpected import error: %v",
			err,
		)
	}

	if !getCalled {
		t.Fatal("expected GET request")
	}

	if len(state) != 1 {
		t.Fatalf(
			"expected one ResourceData result, got %d",
			len(state),
		)
	}

	imported := state[0]

	if got := imported.Id(); got != "secret-123" {
		t.Errorf(
			"expected ID secret-123, got %q",
			got,
		)
	}

	if got := imported.Get("app_id"); got != "app-123" {
		t.Errorf(
			"expected app_id app-123, got %v",
			got,
		)
	}

	if got := imported.Get("env"); got != "production" {
		t.Errorf(
			"expected env production, got %v",
			got,
		)
	}

	if got := imported.Get("key"); got != "DATABASE_URL" {
		t.Errorf(
			"expected key DATABASE_URL, got %v",
			got,
		)
	}

	if got := imported.Get("value"); got != "postgres://localhost" {
		t.Errorf(
			"expected value postgres://localhost, got %v",
			got,
		)
	}

	if got := imported.Get("path"); got != "/database/" {
		t.Errorf(
			"expected path /database/, got %v",
			got,
		)
	}

	if got := imported.Get("comment"); got != "database connection" {
		t.Errorf(
			"expected comment database connection, got %v",
			got,
		)
	}

	tags := imported.Get("tags").([]any)

	if !reflect.DeepEqual(
		tags,
		[]any{"database"},
	) {
		t.Errorf(
			"expected tags [database], got %v",
			tags,
		)
	}

	if got := imported.Get("version"); got != 5 {
		t.Errorf(
			"expected version 5, got %v",
			got,
		)
	}
}

func TestResourceSecretImportStateWithOverride(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-override",
				Key:   "API_KEY",
				Value: "normal-value",
				Override: &secrets.SecretOverride{
					Value:    "override-value",
					IsActive: true,
				},
				Version:   3,
				CreatedAt: "2026-09-18T10:00:00Z",
				UpdatedAt: "2026-09-18T11:00:00Z",
			},
		})
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		nil,
	)

	d.SetId(
		"app-123:production:/:API_KEY",
	)

	state, err := resourceSecretImportState(
		context.Background(),
		d,
		phaseClient,
	)

	if err != nil {
		t.Fatalf(
			"unexpected import error: %v",
			err,
		)
	}

	if len(state) != 1 {
		t.Fatalf(
			"expected one ResourceData result, got %d",
			len(state),
		)
	}

	imported := state[0]

	if got := imported.Id(); got != "secret-override" {
		t.Errorf(
			"expected ID secret-override, got %q",
			got,
		)
	}

	if got := imported.Get("value"); got != "override-value" {
		t.Errorf(
			"expected override-value, got %v",
			got,
		)
	}

	overrideSet := imported.Get("override").(*schema.Set)

	if overrideSet.Len() != 1 {
		t.Fatalf(
			"expected one override item, got %d",
			overrideSet.Len(),
		)
	}

	override := overrideSet.List()[0].(map[string]any)

	if override["value"] != "override-value" {
		t.Errorf(
			"expected override-value, got %v",
			override["value"],
		)
	}

	if override["is_active"] != true {
		t.Errorf(
			"expected is_active true, got %v",
			override["is_active"],
		)
	}
}

func TestResourceSecretImportStateInactiveOverride(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsResponse(t, w, []secrets.Secret{
			{
				ID:    "secret-123",
				Key:   "API_KEY",
				Value: "normal-value",
				Override: &secrets.SecretOverride{
					Value:    "override-value",
					IsActive: false,
				},
			},
		})
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		nil,
	)

	d.SetId(
		"app-123:production:/:API_KEY",
	)

	state, err := resourceSecretImportState(
		context.Background(),
		d,
		phaseClient,
	)

	if err != nil {
		t.Fatalf(
			"unexpected import error: %v",
			err,
		)
	}

	imported := state[0]

	if got := imported.Get("value"); got != "normal-value" {
		t.Errorf(
			"expected normal-value, got %v",
			got,
		)
	}

	overrideSet := imported.Get("override").(*schema.Set)

	if overrideSet.Len() != 0 {
		t.Errorf(
			"expected no override items, got %d",
			overrideSet.Len(),
		)
	}
}

func TestResourceSecretImportStateInvalidID(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error(
			"server should not be called for invalid import ID",
		)
	})

	phaseClient := newTestSecretClient(t, handler)

	tests := []string{
		"",
		"app-123",
		"app-123:production",
		"app-123:production:/database",
		":production:/database/:DATABASE_URL",
		"app-123::/database/:DATABASE_URL",
		"app-123:production::DATABASE_URL",
		"app-123:production:/database/:",
	}

	for _, importID := range tests {
		t.Run(
			fmt.Sprintf("%q", importID),
			func(t *testing.T) {
				d := schema.TestResourceDataRaw(
					t,
					Secret().Schema,
					nil,
				)

				d.SetId(importID)

				state, err := resourceSecretImportState(
					context.Background(),
					d,
					phaseClient,
				)

				if err == nil {
					t.Fatal("expected import error")
				}

				if state != nil {
					t.Errorf(
						"expected nil state, got %v",
						state,
					)
				}

				if !strings.Contains(
					err.Error(),
					"unexpected format of ID",
				) {
					t.Errorf(
						"expected unexpected format error, got %v",
						err,
					)
				}
			},
		)
	}
}

func TestResourceSecretImportStateSecretNotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeSecretsResponse(t, w, []secrets.Secret{
			{
				ID:      "other-secret",
				Key:     "OTHER_KEY",
				Value:   "value",
				Path:    "/database/",
				Version: 1,
			},
		})
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		nil,
	)

	d.SetId(
		"app-123:production:/database/:DATABASE_URL",
	)

	state, err := resourceSecretImportState(
		context.Background(),
		d,
		phaseClient,
	)

	if err == nil {
		t.Fatal("expected import error")
	}

	if state != nil {
		t.Errorf(
			"expected nil state, got %v",
			state,
		)
	}

	expected := "no secret found with key 'DATABASE_URL' at path '/database/' in app 'app-123', env 'production'"

	if err.Error() != expected {
		t.Errorf(
			"expected error %q, got %q",
			expected,
			err.Error(),
		)
	}
}

func TestResourceSecretImportStateListError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeErrorResponse(
			w,
			http.StatusBadRequest,
			"bad request",
		)
	})

	phaseClient := newTestSecretClient(t, handler)

	d := schema.TestResourceDataRaw(
		t,
		Secret().Schema,
		nil,
	)

	d.SetId(
		"app-123:production:/database/:DATABASE_URL",
	)

	state, err := resourceSecretImportState(
		context.Background(),
		d,
		phaseClient,
	)

	if err == nil {
		t.Fatal("expected import error")
	}

	if state != nil {
		t.Errorf(
			"expected nil state, got %v",
			state,
		)
	}

	if !strings.Contains(
		err.Error(),
		"error listing secrets for path '/database/' during import",
	) {
		t.Errorf(
			"unexpected error: %v",
			err,
		)
	}
}

func TestSecretImportIDParsing(t *testing.T) {
	importID := "app-123:production:/database/:DATABASE_URL"

	parts := strings.SplitN(importID, ":", 4)

	if len(parts) != 4 {
		t.Fatalf(
			"expected 4 parts, got %d",
			len(parts),
		)
	}

	expected := []string{
		"app-123",
		"production",
		"/database/",
		"DATABASE_URL",
	}

	if !reflect.DeepEqual(parts, expected) {
		t.Errorf(
			"expected %v, got %v",
			expected,
			parts,
		)
	}
}

func TestSecretImportIDUsesFourParts(t *testing.T) {
	importID := "app-123:production:/database/:DATABASE_URL:extra"

	parts := strings.SplitN(importID, ":", 4)

	if len(parts) != 4 {
		t.Fatalf(
			"expected 4 parts, got %d",
			len(parts),
		)
	}

	expectedLastPart := "DATABASE_URL:extra"

	if parts[3] != expectedLastPart {
		t.Errorf(
			"expected last part %q, got %q",
			expectedLastPart,
			parts[3],
		)
	}
}

func TestSecretQueryEncoding(t *testing.T) {
	values := url.Values{}

	values.Set("app_id", "app id")
	values.Set("env", "production env")
	values.Set("path", "/folder path/")

	encoded := values.Encode()

	if !strings.Contains(
		encoded,
		"app_id=app+id",
	) {
		t.Errorf(
			"expected encoded app_id, got %q",
			encoded,
		)
	}

	if !strings.Contains(
		encoded,
		"env=production+env",
	) {
		t.Errorf(
			"expected encoded env, got %q",
			encoded,
		)
	}

	if !strings.Contains(
		encoded,
		"path=%2Ffolder+path%2F",
	) {
		t.Errorf(
			"expected encoded path, got %q",
			encoded,
		)
	}
}
