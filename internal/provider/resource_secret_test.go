package provider

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/phasehq/golang-sdk/v2/phase"
	"github.com/phasehq/golang-sdk/v2/phase/crypto"
	"github.com/phasehq/golang-sdk/v2/phase/misc"
)

const (
	testAppID   = "app-1"
	testEnvID   = "env-1"
	testEnvName = "Development"
)

// mockPhase is an in-memory stand-in for the end-to-end encrypted secrets API
// (/service/secrets/, E2EESecretsView) that the SDK talks to. It stores the
// ciphertext the SDK sends and follows the server's PUT rules: a comment left
// out keeps its value, while tags are always replaced by what is sent (left
// out means none). GET returns tags in reverse order, since the API has no
// fixed tag order.
type mockPhase struct {
	mu       sync.Mutex
	secrets  []map[string]interface{}
	nextID   int
	userData misc.AppKeyResponse

	envPub, envPriv, salt string
}

func randomHex(t *testing.T) string {
	t.Helper()
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return hex.EncodeToString(b)
}

// newMockPhase starts the mock and returns an SDK client for it. The token and
// wrapped keys are generated the way the console does, so the SDK unwraps the
// environment keys and encrypts and decrypts for real.
func newMockPhase(t *testing.T) (*mockPhase, *phase.Phase) {
	t.Helper()

	appKeys, err := crypto.RandomKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	appPub := hex.EncodeToString(appKeys.PublicKey[:])

	// The app private key is split into two shares; the second is wrapped with a key in the token.
	keyshare0 := randomHex(t)
	share0, _ := hex.DecodeString(keyshare0)
	share1, err := crypto.XORBytes(appKeys.SecretKey[:], share0)
	if err != nil {
		t.Fatal(err)
	}
	var unwrapKey [32]byte
	if _, err := rand.Read(unwrapKey[:]); err != nil {
		t.Fatal(err)
	}
	wrappedKeyShare, err := crypto.EncryptRaw(hex.EncodeToString(share1), unwrapKey)
	if err != nil {
		t.Fatal(err)
	}
	token := fmt.Sprintf("pss_service:v2:%s:%s:%s:%s", randomHex(t), appPub, keyshare0, hex.EncodeToString(unwrapKey[:]))

	seed, salt := randomHex(t), randomHex(t)
	envPub, envPriv, err := crypto.GenerateEnvKeyPair(seed)
	if err != nil {
		t.Fatal(err)
	}
	wrappedSeed, err := crypto.EncryptAsymmetric(seed, appPub)
	if err != nil {
		t.Fatal(err)
	}
	wrappedSalt, err := crypto.EncryptAsymmetric(salt, appPub)
	if err != nil {
		t.Fatal(err)
	}

	m := &mockPhase{
		envPub:  envPub,
		envPriv: envPriv,
		salt:    salt,
		userData: misc.AppKeyResponse{
			WrappedKeyShare: hex.EncodeToString(wrappedKeyShare),
			Apps: []misc.App{{
				ID:         testAppID,
				Name:       "app",
				Encryption: "E2E",
				EnvironmentKeys: []misc.EnvironmentKey{{
					ID:          "env-key-1",
					Environment: misc.Environment{ID: testEnvID, Name: testEnvName, EnvType: "dev"},
					IdentityKey: envPub,
					WrappedSeed: wrappedSeed,
					WrappedSalt: wrappedSalt,
				}},
			}},
		},
	}

	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)

	client, err := phase.New(token, srv.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	return m, client
}

func (m *mockPhase) encrypt(t *testing.T, plaintext string) string {
	t.Helper()
	if plaintext == "" {
		return ""
	}
	ct, err := crypto.EncryptAsymmetric(plaintext, m.envPub)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

func (m *mockPhase) decrypt(t *testing.T, ciphertext interface{}) string {
	t.Helper()
	ct, _ := ciphertext.(string)
	if ct == "" {
		return ""
	}
	pt, err := crypto.DecryptAsymmetric(ct, m.envPriv, m.envPub)
	if err != nil {
		t.Fatal(err)
	}
	return pt
}

// mockSecret is a decrypted view of a stored secret.
type mockSecret struct {
	ID, Key, Value, Comment, Path string
	Tags                          []string
}

// seed adds a secret directly, as if it was created outside Terraform.
func (m *mockPhase) seed(t *testing.T, s mockSecret) string {
	t.Helper()
	digest, err := crypto.Blake2bDigest(s.Key, m.salt)
	if err != nil {
		t.Fatal(err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	id := fmt.Sprintf("secret-%d", m.nextID)
	m.secrets = append(m.secrets, map[string]interface{}{
		"id":         id,
		"key":        m.encrypt(t, s.Key),
		"key_digest": digest,
		"value":      m.encrypt(t, s.Value),
		"comment":    m.encrypt(t, s.Comment),
		"path":       s.Path,
		"tags":       toInterfaces(s.Tags),
		"type":       "secret",
		"version":    float64(1),
	})
	return id
}

// get returns the decrypted secret, or nil when it does not exist.
func (m *mockPhase) get(t *testing.T, id string) *mockSecret {
	t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.secrets {
		if s["id"] == id {
			return &mockSecret{
				ID:      id,
				Key:     m.decrypt(t, s["key"]),
				Value:   m.decrypt(t, s["value"]),
				Comment: m.decrypt(t, s["comment"]),
				Path:    s["path"].(string),
				Tags:    stringList(s["tags"]),
			}
		}
	}
	return nil
}

func (m *mockPhase) remove(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.secrets {
		if s["id"] == id {
			m.secrets = append(m.secrets[:i], m.secrets[i+1:]...)
			return true
		}
	}
	return false
}

func (m *mockPhase) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch {
	case r.URL.Path == "/service/secrets/tokens/" && r.Method == http.MethodGet:
		writeJSON(w, http.StatusOK, m.userData)
	case r.URL.Path != "/service/secrets/":
		http.NotFound(w, r)
	case r.Header.Get("Environment") != testEnvID:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "Environment not found"})
	case r.Method == http.MethodGet:
		m.list(w, r)
	case r.Method == http.MethodPost:
		m.create(w, r)
	case r.Method == http.MethodPut:
		m.update(w, r)
	case r.Method == http.MethodDelete:
		m.delete(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (m *mockPhase) list(w http.ResponseWriter, r *http.Request) {
	path := r.Header.Get("Path")
	digest := r.Header.Get("KeyDigest")

	out := make([]map[string]interface{}, 0)
	for _, s := range m.secrets {
		if (path != "" && s["path"] != path) || (digest != "" && s["key_digest"] != digest) {
			continue
		}
		c := make(map[string]interface{}, len(s))
		for k, v := range s {
			c[k] = v
		}
		tags := slices.Clone(s["tags"].([]interface{}))
		slices.Reverse(tags)
		c["tags"] = tags
		out = append(out, c)
	}
	writeJSON(w, http.StatusOK, out)
}

// duplicate reports whether another secret has the same key digest at the path.
func (m *mockPhase) duplicate(id string, path, digest interface{}) bool {
	for _, s := range m.secrets {
		if s["id"] != id && s["path"] == path && s["key_digest"] == digest {
			return true
		}
	}
	return false
}

func (m *mockPhase) create(w http.ResponseWriter, r *http.Request) {
	entries, ok := decodeSecrets(w, r)
	if !ok {
		return
	}
	for _, e := range entries {
		path, _ := e["path"].(string)
		if path == "" {
			path = "/"
		}
		if m.duplicate("", path, e["keyDigest"]) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "Duplicate secret found"})
			return
		}
		m.nextID++
		secretType, _ := e["type"].(string)
		if secretType == "" {
			secretType = "secret"
		}
		m.secrets = append(m.secrets, map[string]interface{}{
			"id":         fmt.Sprintf("secret-%d", m.nextID),
			"key":        e["key"],
			"key_digest": e["keyDigest"],
			"value":      e["value"],
			"comment":    e["comment"],
			"path":       path,
			"tags":       tagsOrNone(e["tags"]),
			"type":       secretType,
			"version":    float64(1),
		})
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Created"})
}

func (m *mockPhase) update(w http.ResponseWriter, r *http.Request) {
	entries, ok := decodeSecrets(w, r)
	if !ok {
		return
	}
	for _, e := range entries {
		var s map[string]interface{}
		for _, existing := range m.secrets {
			if existing["id"] == e["id"] {
				s = existing
			}
		}
		if s == nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "Secret not found"})
			return
		}

		path := s["path"]
		if p, ok := e["path"].(string); ok && p != "" {
			path = p
		}
		digest := s["key_digest"]
		if d, ok := e["keyDigest"]; ok {
			digest = d
		}
		if m.duplicate(s["id"].(string), path, digest) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "Duplicate secret found"})
			return
		}

		for _, field := range []string{"key", "value", "comment", "type"} {
			if v, ok := e[field]; ok {
				s[field] = v
			}
		}
		s["key_digest"] = digest
		s["path"] = path
		// The server always replaces tags with what was sent, so left out clears them.
		s["tags"] = tagsOrNone(e["tags"])
		s["version"] = s["version"].(float64) + 1
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "Updated"})
}

func (m *mockPhase) delete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Secrets []string `json:"secrets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	// The server ignores IDs that no longer exist.
	kept := m.secrets[:0]
	for _, s := range m.secrets {
		if !slices.Contains(body.Secrets, s["id"].(string)) {
			kept = append(kept, s)
		}
	}
	m.secrets = kept
	writeJSON(w, http.StatusOK, map[string]string{"message": "Deleted"})
}

func decodeSecrets(w http.ResponseWriter, r *http.Request) ([]map[string]interface{}, bool) {
	var body struct {
		Secrets []map[string]interface{} `json:"secrets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Secrets) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "'secrets' must be a non-empty list."})
		return nil, false
	}
	return body.Secrets, true
}

func tagsOrNone(v interface{}) []interface{} {
	tags, _ := v.([]interface{})
	if tags == nil {
		tags = []interface{}{}
	}
	return tags
}

func toInterfaces(s []string) []interface{} {
	out := make([]interface{}, 0, len(s))
	for _, v := range s {
		out = append(out, v)
	}
	return out
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Terraform lifecycle helpers, driving the resource the way core does.

func secretConfig(attrs map[string]interface{}) map[string]interface{} {
	cfg := map[string]interface{}{
		"app_id": testAppID,
		"env":    testEnvName,
		"key":    "API_KEY",
		"value":  "secret-value",
	}
	for k, v := range attrs {
		cfg[k] = v
	}
	return cfg
}

func plan(t *testing.T, client *phase.Phase, state *terraform.InstanceState, cfg map[string]interface{}) *terraform.InstanceDiff {
	t.Helper()
	diff, err := resourceSecret().Diff(context.Background(), state, terraform.NewResourceConfigRaw(cfg), client)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	return diff
}

func apply(t *testing.T, client *phase.Phase, state *terraform.InstanceState, cfg map[string]interface{}) *terraform.InstanceState {
	t.Helper()
	diff := plan(t, client, state, cfg)
	newState, diags := resourceSecret().Apply(context.Background(), state, diff, client)
	if diags.HasError() {
		t.Fatalf("apply: %v", diags)
	}
	return newState
}

// refresh returns nil when the resource was removed from state.
func refresh(t *testing.T, client *phase.Phase, state *terraform.InstanceState) *terraform.InstanceState {
	t.Helper()
	newState, diags := resourceSecret().RefreshWithoutUpgrade(context.Background(), state, client)
	if diags.HasError() {
		t.Fatalf("refresh: %v", diags)
	}
	return newState
}

func assertNoChanges(t *testing.T, client *phase.Phase, state *terraform.InstanceState, cfg map[string]interface{}) {
	t.Helper()
	if diff := plan(t, client, state, cfg); !diff.Empty() {
		for k, a := range diff.Attributes {
			t.Errorf("%s: %q => %q", k, a.Old, a.New)
		}
		t.FailNow()
	}
}

// Tests

// The provider relies on these SDK update semantics.
func TestSDKUpdateTagsAndCommentSemantics(t *testing.T) {
	m, client := newMockPhase(t)
	id := m.seed(t, mockSecret{Key: "API_KEY", Value: "v1", Path: "/", Comment: "note", Tags: []string{"a", "b"}})

	update := func(comment *string, tags []string) *mockSecret {
		t.Helper()
		err := client.Update(phase.UpdateOptions{AppID: testAppID, EnvName: testEnvName, ID: id, SourcePath: "/", Value: "v2", Comment: comment, Tags: tags})
		if err != nil {
			t.Fatal(err)
		}
		return m.get(t, id)
	}

	got := update(nil, nil)
	if got.Comment != "note" || len(got.Tags) != 2 || !slices.Contains(got.Tags, "a") || !slices.Contains(got.Tags, "b") {
		t.Fatalf("nil comment/tags should be kept, got comment=%q tags=%v", got.Comment, got.Tags)
	}

	empty := ""
	got = update(&empty, []string{})
	if got.Comment != "" || len(got.Tags) != 0 {
		t.Fatalf("\"\" and [] should clear, got comment=%q tags=%v", got.Comment, got.Tags)
	}
	if got.Value != "v2" {
		t.Fatalf("value not updated, got %q", got.Value)
	}
}

func TestResourceSecretUpdateClearsTagsAndComment(t *testing.T) {
	m, client := newMockPhase(t)

	cfg := secretConfig(map[string]interface{}{
		"comment": "note",
		"tags":    []interface{}{"a"},
	})
	state := apply(t, client, nil, cfg)
	assertNoChanges(t, client, refresh(t, client, state), cfg)

	// Remove tags and comment from config.
	cfg = secretConfig(nil)
	state = apply(t, client, state, cfg)

	got := m.get(t, state.ID)
	if got.Comment != "" || len(got.Tags) != 0 {
		t.Fatalf("expected tags and comment cleared in Phase, got comment=%q tags=%v", got.Comment, got.Tags)
	}
	assertNoChanges(t, client, refresh(t, client, state), cfg)
}

func TestResourceSecretUpdateKeepsUnchangedTags(t *testing.T) {
	m, client := newMockPhase(t)

	cfg := secretConfig(map[string]interface{}{"tags": []interface{}{"a"}, "comment": "note"})
	state := apply(t, client, nil, cfg)

	cfg = secretConfig(map[string]interface{}{"tags": []interface{}{"a"}, "comment": "note", "value": "new-value"})
	state = apply(t, client, state, cfg)

	got := m.get(t, state.ID)
	if got.Value != "new-value" || got.Comment != "note" || !slices.Equal(got.Tags, []string{"a"}) {
		t.Fatalf("unexpected secret after update: %+v", got)
	}
	assertNoChanges(t, client, refresh(t, client, state), cfg)
}

func TestResourceSecretTagOrderNoDiff(t *testing.T) {
	_, client := newMockPhase(t)

	// The mock returns tags in reverse order.
	cfg := secretConfig(map[string]interface{}{"tags": []interface{}{"a", "b", "c"}})
	state := apply(t, client, nil, cfg)

	assertNoChanges(t, client, refresh(t, client, state), cfg)
}

func TestResourceSecretReadMatchesID(t *testing.T) {
	m, client := newMockPhase(t)

	// Same key at another path, stored first.
	other := m.seed(t, mockSecret{Key: "API_KEY", Value: "other-value", Path: "/other"})

	cfg := secretConfig(nil)
	state := apply(t, client, nil, cfg)
	if state.ID == other {
		t.Fatalf("state picked up the secret at /other")
	}

	refreshed := refresh(t, client, state)
	if refreshed.ID != state.ID || refreshed.Attributes["path"] != "/" || refreshed.Attributes["value"] != "secret-value" {
		t.Fatalf("refresh read the wrong secret: id=%s path=%q", refreshed.ID, refreshed.Attributes["path"])
	}
	assertNoChanges(t, client, refreshed, cfg)
}

func TestResourceSecretReadDeletedOutsideTerraform(t *testing.T) {
	m, client := newMockPhase(t)

	cfg := secretConfig(map[string]interface{}{"tags": []interface{}{"a"}})
	state := apply(t, client, nil, cfg)

	if !m.remove(state.ID) {
		t.Fatalf("secret %s not in mock", state.ID)
	}
	if refreshed := refresh(t, client, state); refreshed != nil {
		t.Fatalf("expected secret removed from state, got ID %q", refreshed.ID)
	}

	// The next apply recreates it.
	state = apply(t, client, nil, cfg)
	if m.get(t, state.ID) == nil {
		t.Fatalf("secret was not recreated")
	}
}

func TestResourceSecretCreateConflictAdoptsSamePath(t *testing.T) {
	m, client := newMockPhase(t)

	// Same key at another path, stored first.
	other := m.seed(t, mockSecret{Key: "API_KEY", Value: "other-value", Path: "/other", Comment: "other", Tags: []string{"x"}})
	// Created outside Terraform at the configured path, so create returns 409.
	existing := m.seed(t, mockSecret{Key: "API_KEY", Value: "old-value", Path: "/"})

	cfg := secretConfig(map[string]interface{}{"tags": []interface{}{"a"}})
	state := apply(t, client, nil, cfg)

	if state.ID != existing {
		t.Fatalf("expected to adopt %s at /, got %s", existing, state.ID)
	}
	if got := m.get(t, existing); got.Value != "secret-value" || !slices.Equal(got.Tags, []string{"a"}) {
		t.Fatalf("existing secret not updated: %+v", got)
	}
	if got := m.get(t, other); got.Path != "/other" || got.Value != "other-value" || got.Comment != "other" || !slices.Equal(got.Tags, []string{"x"}) {
		t.Fatalf("secret at /other was modified: %+v", got)
	}
	assertNoChanges(t, client, refresh(t, client, state), cfg)
}
