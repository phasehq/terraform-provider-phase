package applications

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/phasehq/terraform-provider/internal/client"
)

func UpdateApplication(c *client.PhaseClient, appID string, app Application) (*Application, error) {
	url := fmt.Sprintf("%s/v1/apps/%s/", c.HostURL, appID)

	body, err := json.Marshal(app)
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
			return nil, fmt.Errorf("failed to update application: %w", apiErr)
		}
		return nil, err
	}

	var updatedApp *Application
	err = json.Unmarshal(responseBody, &updatedApp)
	if err != nil {
		return nil, err
	}

	if updatedApp == nil {
		return nil, fmt.Errorf("no app updated")
	}

	return updatedApp, nil
}

func DeleteApplication(c *client.PhaseClient, appID string) error {
	url := fmt.Sprintf("%s/v1/apps/%s/", c.HostURL, appID)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	_, err = c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return fmt.Errorf("failed to delete application: %w", apiErr)
		}
		return err
	}

	return nil
}

func CreateApplication(c *client.PhaseClient, app CreateApplicationRequest) (*Application, error) {
	url := fmt.Sprintf("%s/v1/apps/", c.HostURL)

	body, err := json.Marshal(app)
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
			return nil, fmt.Errorf("failed to create application: %w", apiErr)
		}
		return nil, err
	}

	var application Application
	err = json.Unmarshal(responseBody, &application)
	if err != nil {
		return nil, err
	}

	if application.ID == "" {
		return nil, fmt.Errorf("create application response did not contain an application ID")
	}

	return &application, nil
}

func ReadApplication(c *client.PhaseClient, id string) (*Application, error) {
	url := fmt.Sprintf("%s/v1/apps/%s/", c.HostURL, id)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	responseBody, err := c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("failed to read application: %w", apiErr)
		}
		return nil, err
	}

	var response *Application
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, err
	}
	return response, nil
}

func ListApplications(c *client.PhaseClient) ([]Application, error) {
	url := fmt.Sprintf("%s/v1/apps/", c.HostURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	responseBody, err := c.Do(req)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) {
			return nil, fmt.Errorf("failed to list application: %w", apiErr)
		}
		return nil, err
	}

	var response struct {
		Data []Application `json:"data"`
	}
	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return nil, err
	}
	return response.Data, nil
}
