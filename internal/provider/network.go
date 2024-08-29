package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// setHeaders sets the common headers for all requests
func (c *PhaseClient) setHeaders(req *http.Request, tokenType string) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, c.Token))
	req.Header.Set("User-Agent", UserAgent)
}

// CreateSecret creates a new secret
func (c *PhaseClient) CreateSecret(appID, env, tokenType string, secret Secret) (*Secret, error) {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s", c.HostURL, appID, env)

	body, err := json.Marshal(map[string]interface{}{
		"secrets": []Secret{secret},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, tokenType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to create secret: %s", resp.Status)
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

// ReadSecret reads a secret by its ID
func (c *PhaseClient) ReadSecret(appID, env, secretID, tokenType string) (*Secret, error) {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s&id=%s", c.HostURL, appID, env, secretID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, tokenType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to read secret: %s", resp.Status)
	}

	var secrets []Secret
	err = json.Unmarshal(responseBody, &secrets)
	if err != nil {
		return nil, err
	}

	if len(secrets) == 0 {
		return nil, fmt.Errorf("secret not found")
	}

	return &secrets[0], nil
}

// UpdateSecret updates an existing secret
func (c *PhaseClient) UpdateSecret(appID, env, tokenType string, secret Secret) (*Secret, error) {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s", c.HostURL, appID, env)

	body, err := json.Marshal(map[string]interface{}{
		"secrets": []Secret{secret},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, tokenType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to update secret: %s", resp.Status)
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
func (c *PhaseClient) DeleteSecret(appID, env, secretID, tokenType string) error {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s", c.HostURL, appID, env)

	body, err := json.Marshal(map[string]interface{}{
		"secrets": []string{secretID},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	c.setHeaders(req, tokenType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete secret: %s", resp.Status)
	}

	return nil
}

// ListSecrets lists all secrets for a given app, environment, and path
func (c *PhaseClient) ListSecrets(appID, env, path, tokenType string) ([]Secret, error) {
	url := fmt.Sprintf("%s/v1/secrets/?app_id=%s&env=%s&path=%s", c.HostURL, appID, env, path)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	c.setHeaders(req, tokenType)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to list secrets: %s", resp.Status)
	}

	var secrets []Secret
	err = json.Unmarshal(responseBody, &secrets)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}