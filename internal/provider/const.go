package provider

import "net/http"

const (
	// Version of the provider
	Version = "0.1.0"

	// DefaultHostURL is the default host for Phase API
	DefaultHostURL = "https://api.phase.dev"

	// UserAgent is the user agent for the provider
	UserAgent = "terraform-provider-phase/" + Version
)

// PhaseClient represents the client for interacting with the Phase API
type PhaseClient struct {
	HostURL    string
	HTTPClient *http.Client
	Token      string
}

// Secret represents a secret in the Phase API
type Secret struct {
	ID      string   `json:"id,omitempty"`
	Key     string   `json:"key"`
	Value   string   `json:"value"`
	Comment string   `json:"comment,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Path    string   `json:"path,omitempty"`
}