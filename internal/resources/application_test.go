package resources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/terraform-provider/internal/client"
)

func newApplicationTestClient(server *httptest.Server) *client.PhaseClient {
	return &client.PhaseClient{
		HostURL:    server.URL,
		HTTPClient: server.Client(),
		Token:      "test-token",
		TokenType:  "User",
	}
}

func TestApplication(t *testing.T) {
	resource := Application()

	if resource == nil {
		t.Fatal("expected resource, got nil")
	}

	if resource.CreateContext == nil {
		t.Error("expected CreateContext to be defined")
	}

	if resource.ReadContext == nil {
		t.Error("expected ReadContext to be defined")
	}

	if resource.UpdateContext == nil {
		t.Error("expected UpdateContext to be defined")
	}

	if resource.DeleteContext == nil {
		t.Error("expected DeleteContext to be defined")
	}

	tests := []struct {
		name      string
		field     string
		required  bool
		optional  bool
		fieldType schema.ValueType
	}{
		{
			name:      "name",
			field:     "name",
			required:  true,
			fieldType: schema.TypeString,
		},
		{
			name:      "description",
			field:     "description",
			optional:  true,
			fieldType: schema.TypeString,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, ok := resource.Schema[tt.field]
			if !ok {
				t.Fatalf("expected field %q to exist", tt.field)
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
		})
	}
}

func TestResourceApplicationCreate(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedID   string
		expectedName string
		expectedDesc string
		expectedErr  bool
	}{
		{
			name:         "success",
			statusCode:   http.StatusCreated,
			responseBody: `{"id":"app-123","name":"test-app","description":"test description"}`,
			expectedID:   "app-123",
			expectedName: "test-app",
			expectedDesc: "test description",
		},
		{
			name:         "api error",
			statusCode:   http.StatusBadRequest,
			responseBody: `{"message":"invalid application"}`,
			expectedErr:  true,
		},
		{
			name:         "missing application id",
			statusCode:   http.StatusCreated,
			responseBody: `{"name":"test-app","description":"test description"}`,
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}

				if r.URL.Path != "/v1/apps/" {
					t.Errorf(
						"expected /v1/apps/, got %s",
						r.URL.Path,
					)
				}

				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Fatalf("failed to decode request body: %v", err)
				}

				if tt.name == "success" {
					if body["name"] != "test-app" {
						t.Errorf(
							"expected name test-app, got %v",
							body["name"],
						)
					}

					if body["description"] != "test description" {
						t.Errorf(
							"expected description test description, got %v",
							body["description"],
						)
					}
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			resource := Application()

			data := schema.TestResourceDataRaw(
				t,
				resource.Schema,
				map[string]any{
					"name":        "test-app",
					"description": "test description",
				},
			)

			client := newApplicationTestClient(server)

			diags := resourceApplicationCreate(
				context.Background(),
				data,
				client,
			)

			if tt.expectedErr {
				if !diags.HasError() {
					t.Fatal("expected diagnostics error, got none")
				}

				return
			}

			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if data.Id() != tt.expectedID {
				t.Errorf(
					"expected ID %s, got %s",
					tt.expectedID,
					data.Id(),
				)
			}

			name, ok := data.GetOk("name")
			if !ok {
				t.Fatal("expected name in state")
			}

			if name.(string) != tt.expectedName {
				t.Errorf(
					"expected name %s, got %s",
					tt.expectedName,
					name.(string),
				)
			}

			description, ok := data.GetOk("description")
			if !ok {
				t.Fatal("expected description in state")
			}

			if description.(string) != tt.expectedDesc {
				t.Errorf(
					"expected description %s, got %s",
					tt.expectedDesc,
					description.(string),
				)
			}
		})
	}
}

func TestResourceApplicationCreateWithoutDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["name"] != "test-app" {
			t.Errorf(
				"expected name test-app, got %v",
				body["name"],
			)
		}

		if _, exists := body["description"]; exists {
			t.Error("expected description to be omitted")
		}

		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{
			"id":"app-123",
			"name":"test-app"
		}`))
	}))
	defer server.Close()

	resource := Application()

	data := schema.TestResourceDataRaw(
		t,
		resource.Schema,
		map[string]any{
			"name": "test-app",
		},
	)

	client := newApplicationTestClient(server)

	diags := resourceApplicationCreate(
		context.Background(),
		data,
		client,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Id() != "app-123" {
		t.Errorf("expected ID app-123, got %s", data.Id())
	}
}

func TestResourceApplicationRead(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		responseBody string
		expectedID   string
		expectedName string
		expectedDesc string
		expectedErr  bool
	}{
		{
			name:         "success",
			statusCode:   http.StatusOK,
			responseBody: `{"id":"app-123","name":"test-app","description":"test description"}`,
			expectedID:   "app-123",
			expectedName: "test-app",
			expectedDesc: "test description",
		},
		{
			name:         "api error",
			statusCode:   http.StatusNotFound,
			responseBody: `{"message":"application not found"}`,
			expectedErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet {
					t.Errorf("expected GET, got %s", r.Method)
				}

				if r.URL.Path != "/v1/apps/app-123/" {
					t.Errorf(
						"expected /v1/apps/app-123/, got %s",
						r.URL.Path,
					)
				}

				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			resource := Application()

			data := schema.TestResourceDataRaw(
				t,
				resource.Schema,
				map[string]any{
					"name":        "old-name",
					"description": "old description",
				},
			)

			data.SetId("app-123")

			client := newApplicationTestClient(server)

			diags := resourceApplicationRead(
				context.Background(),
				data,
				client,
			)

			if tt.expectedErr {
				if !diags.HasError() {
					t.Fatal("expected diagnostics error, got none")
				}

				return
			}

			if diags.HasError() {
				t.Fatalf("unexpected diagnostics: %v", diags)
			}

			if data.Id() != tt.expectedID {
				t.Errorf(
					"expected ID %s, got %s",
					tt.expectedID,
					data.Id(),
				)
			}

			name := data.Get("name").(string)
			if name != tt.expectedName {
				t.Errorf(
					"expected name %s, got %s",
					tt.expectedName,
					name,
				)
			}

			description := data.Get("description").(string)
			if description != tt.expectedDesc {
				t.Errorf(
					"expected description %s, got %s",
					tt.expectedDesc,
					description,
				)
			}
		})
	}
}

func TestResourceApplicationReadWithEmptyApplication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`null`))
	}))
	defer server.Close()

	resource := Application()

	data := schema.TestResourceDataRaw(
		t,
		resource.Schema,
		map[string]any{},
	)

	data.SetId("app-123")

	client := newApplicationTestClient(server)

	diags := resourceApplicationRead(
		context.Background(),
		data,
		client,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error, got none")
	}

	if !strings.Contains(diags[0].Summary, "No app found") {
		t.Errorf(
			"expected 'No app found', got %s",
			diags[0].Summary,
		)
	}
}

func TestResourceApplicationUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT, got %s", r.Method)
		}

		if r.URL.Path != "/v1/apps/app-123/" {
			t.Errorf(
				"expected /v1/apps/app-123/, got %s",
				r.URL.Path,
			)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["id"] != "app-123" {
			t.Errorf("expected ID app-123, got %v", body["id"])
		}

		if body["name"] != "updated-app" {
			t.Errorf(
				"expected name updated-app, got %v",
				body["name"],
			)
		}

		if body["description"] != "updated description" {
			t.Errorf(
				"expected description updated description, got %v",
				body["description"],
			)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"app-123",
			"name":"updated-app",
			"description":"updated description"
		}`))
	}))
	defer server.Close()

	resource := Application()

	data := schema.TestResourceDataRaw(
		t,
		resource.Schema,
		map[string]any{
			"name":        "updated-app",
			"description": "updated description",
		},
	)

	data.SetId("app-123")

	client := newApplicationTestClient(server)

	diags := resourceApplicationUpdate(
		context.Background(),
		data,
		client,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Id() != "app-123" {
		t.Errorf(
			"expected ID app-123, got %s",
			data.Id(),
		)
	}
}

func TestResourceApplicationUpdateWithoutDescription(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}

		if body["name"] != "updated-app" {
			t.Errorf(
				"expected name updated-app, got %v",
				body["name"],
			)
		}

		if _, exists := body["description"]; exists {
			t.Error("expected description to be omitted")
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"id":"app-123",
			"name":"updated-app"
		}`))
	}))
	defer server.Close()

	resource := Application()

	data := schema.TestResourceDataRaw(
		t,
		resource.Schema,
		map[string]any{
			"name": "updated-app",
		},
	)

	data.SetId("app-123")

	client := newApplicationTestClient(server)

	diags := resourceApplicationUpdate(
		context.Background(),
		data,
		client,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
}

func TestResourceApplicationDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}

		if r.URL.Path != "/v1/apps/app-123/" {
			t.Errorf(
				"expected /v1/apps/app-123/, got %s",
				r.URL.Path,
			)
		}

		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	resource := Application()

	data := schema.TestResourceDataRaw(
		t,
		resource.Schema,
		map[string]any{},
	)

	data.SetId("app-123")

	client := newApplicationTestClient(server)

	diags := resourceApplicationDelete(
		context.Background(),
		data,
		client,
	)

	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}

	if data.Id() != "" {
		t.Errorf(
			"expected ID to be empty after delete, got %s",
			data.Id(),
		)
	}
}

func TestResourceApplicationDeleteError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"application not found"}`))
	}))
	defer server.Close()

	resource := Application()

	data := schema.TestResourceDataRaw(
		t,
		resource.Schema,
		map[string]any{
			"app_id": "app-123",
		},
	)

	data.SetId("app-123")

	client := newApplicationTestClient(server)

	diags := resourceApplicationDelete(
		context.Background(),
		data,
		client,
	)

	if !diags.HasError() {
		t.Fatal("expected diagnostics error, got none")
	}

	if data.Id() != "app-123" {
		t.Errorf(
			"expected ID to remain app-123 after failed delete, got %s",
			data.Id(),
		)
	}
}

func TestResourceApplicationImportState(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		wantErr    bool
		expectedID string
	}{
		{
			name:       "valid application ID",
			id:         "app-123",
			expectedID: "app-123",
		},
		{
			name:    "empty application ID",
			id:      "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(
				t,
				Application().Schema,
				nil,
			)
			d.SetId(tt.id)

			got, err := resourceApplicationImportState(
				context.Background(),
				d,
				nil,
			)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != 1 {
				t.Fatalf("expected 1 resource data, got %d", len(got))
			}

			if got[0].Id() != tt.expectedID {
				t.Errorf(
					"expected ID %q, got %q",
					tt.expectedID,
					got[0].Id(),
				)
			}
		})
	}
}
