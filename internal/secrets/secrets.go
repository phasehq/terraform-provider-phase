package secrets

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/phasehq/terraform-provider/internal/client"
)

// Secret represents a secret in the Phase API
type Secret struct {
	ID        string          `json:"id,omitempty"`
	Key       string          `json:"key"`
	Value     string          `json:"value"`
	Comment   string          `json:"comment,omitempty"`
	Path      string          `json:"path,omitempty"`
	Tags      []string        `json:"tags,omitempty"`
	Version   int             `json:"version,omitempty"`
	KeyDigest string          `json:"keyDigest,omitempty"`
	CreatedAt string          `json:"createdAt,omitempty"`
	UpdatedAt string          `json:"updatedAt,omitempty"`
	Override  *SecretOverride `json:"override,omitempty"`
}

// SecretOverride represents a personal secret override
type SecretOverride struct {
	ID       string `json:"id,omitempty"`
	Value    string `json:"value"`
	IsActive bool   `json:"isActive"`
}

// CreateSecret creates a new secret
func CreateSecret(c *client.PhaseClient, appID, env string, secret Secret) (*Secret, error) {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s", c.HostURL, appID, env)

	body, err := json.Marshal(map[string]any{
		"secrets": []Secret{secret},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	responseBody, err := c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("failed to create secret: %w", apiErr)
		}
		return nil, err
	}

	var createdSecrets []Secret
	err = json.Unmarshal(responseBody, &createdSecrets)
	if err != nil {
		return nil, err
	}

	if len(createdSecrets) == 0 {
		return nil, fmt.Errorf("no secret created")
	}

	return &createdSecrets[0], nil
}

// If secretKey is empty, it fetches all secrets for the given app and environment.
func ReadSecret(c *client.PhaseClient, appID, env, secretKey string, tags ...string) ([]Secret, error) {
	var url string
	if secretKey != "" {
		url = fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s&key=%s", c.HostURL, appID, env, secretKey)
	} else {
		url = fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s", c.HostURL, appID, env)
	}

	// Add tags filter if provided
	if len(tags) > 0 && tags[0] != "" {
		url = fmt.Sprintf("%s&tags=%s", url, strings.Join(tags, ","))
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	responseBody, err := c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("failed to read secret(s): %w", apiErr)
		}
		return nil, err
	}

	var secrets []Secret
	err = json.Unmarshal(responseBody, &secrets)
	if err != nil {
		return nil, err
	}

	if len(secrets) == 0 {
		return nil, fmt.Errorf("no secrets found")
	}

	return secrets, nil
}

// UpdateSecret updates an existing secret
func UpdateSecret(c *client.PhaseClient, appID, env string, secret Secret) (*Secret, error) {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s", c.HostURL, appID, env)

	body, err := json.Marshal(map[string]any{
		"secrets": []Secret{secret},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	responseBody, err := c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("failed to update secret: %w", apiErr)
		}
		return nil, err
	}

	var updatedSecrets []Secret
	err = json.Unmarshal(responseBody, &updatedSecrets)
	if err != nil {
		return nil, err
	}

	if len(updatedSecrets) == 0 {
		return nil, fmt.Errorf("no secret updated")
	}

	return &updatedSecrets[0], nil
}

// DeleteSecret deletes a secret by its ID
func DeleteSecret(c *client.PhaseClient, appID, env, secretID string) error {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s", c.HostURL, appID, env)

	body, err := json.Marshal(map[string]any{
		"secrets": []string{secretID},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	_, err = c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return fmt.Errorf("failed to delete secret: %w", apiErr)
		}
		return err
	}

	return nil
}

// ListSecrets lists all secrets for a given app, environment, and path
func ListSecrets(c *client.PhaseClient, appID, env, path string) ([]Secret, error) {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s&path=%s", c.HostURL, appID, env, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	responseBody, err := c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("failed to list secrets: %w", apiErr)
		}
		return nil, err
	}

	var secrets []Secret
	err = json.Unmarshal(responseBody, &secrets)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}
