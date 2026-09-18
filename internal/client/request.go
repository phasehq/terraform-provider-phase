package client

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"runtime"
	"strings"

	"github.com/phasehq/terraform-provider/internal/config"
)

// setHeaders sets the common headers for all requests
func (c *PhaseClient) setHeaders(req *http.Request) {
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

	userAgent := fmt.Sprintf("%s (%s)", config.UserAgent, strings.Join(details, "; "))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s %s", c.TokenType, c.Token))
	req.Header.Set("User-Agent", userAgent)
}

func (c *PhaseClient) Do(req *http.Request) ([]byte, error) {
	c.setHeaders(req)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			Body:       body,
		}
	}

	return body, nil
}
