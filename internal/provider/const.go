package provider

import (
	"net/http"
	"regexp"
)

const (
	// Version of the provider
	Version = "0.2.0"

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
	TokenType  string
}

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

var (
	// Compiled regex patterns
	PssUserPattern    = regexp.MustCompile(`^pss_user:v(\d+):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64})$`)
	PssServicePattern = regexp.MustCompile(`^pss_service:v(\d+):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64}):([a-fA-F0-9]{64})$`)
)
