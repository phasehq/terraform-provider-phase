package provider

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"strings"
)

// setHeaders sets the common headers for all requests
func (c *PhaseClient) setHeaders(req *http.Request, tokenType string) {
	osType := runtime.GOOS
	architecture := runtime.GOARCH

	details := []string{fmt.Sprintf("%s %s", osType, architecture)}

	currentUser, err := user.Current()
	if err == nil {
		hostname, err := os.Hostname()
		if err == nil {
			userHostString := fmt.Sprintf("%s@%s", currentUser.Username, hostname)
			details = append(details, userHostString)
		}
	}

	userAgent := fmt.Sprintf("%s (%s)", UserAgent, strings.Join(details, "; "))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, c.Token))
	req.Header.Set("User-Agent", userAgent)

	if c.SkipTLSVerification {
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
		c.HTTPClient.Transport = transport
	}

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
		return nil, fmt.Errorf("failed to create secret: %s - %s", resp.Status, string(responseBody))
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
func (c *PhaseClient) ReadSecret(appID, env, secretKey, tokenType string, tags ...string) ([]Secret, error) {
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
		return nil, fmt.Errorf("failed to read secret(s): %s - %s", resp.Status, string(responseBody))
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
		return nil, fmt.Errorf("failed to update secret: %s - %s", resp.Status, string(responseBody))
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

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to delete secret: %s - %s", resp.Status, string(responseBody))
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
		return nil, fmt.Errorf("failed to list secrets: %s - %s", resp.Status, string(responseBody))
	}

	var secrets []Secret
	err = json.Unmarshal(responseBody, &secrets)
	if err != nil {
		return nil, err
	}

	return secrets, nil
}
