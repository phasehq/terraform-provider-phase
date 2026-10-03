package provider

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/v2/phase"
	"github.com/phasehq/golang-sdk/v2/phase/misc"
	"github.com/phasehq/golang-sdk/v2/phase/network"
)

// sdkUserAgent is captured before SetUserAgent is called so the SDK's own identifier is kept.
var sdkUserAgent = network.GetUserAgent()

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHASE_HOST", DefaultHostURL),
				Description: "The host URL for the Phase API. Can be set with PHASE_HOST environment variable.",
			},
			"phase_token": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.MultiEnvDefaultFunc([]string{"PHASE_TOKEN", "PHASE_SERVICE_TOKEN", "PHASE_PAT_TOKEN"}, nil),
				Description: "The token for authenticating with Phase. Can be a service token or a personal access token (PAT). Can be set with PHASE_TOKEN, PHASE_SERVICE_TOKEN, or PHASE_PAT_TOKEN environment variables.",
			},
			"skip_tls_verification": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip SSL/TLS certificate validation for the PHASE_HOST. Applies to every Phase provider configuration in the same Terraform run. Defaults to false.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"phase_secret":                resourceSecret(),
			"phase_app":                   resourceApp(),
			"phase_environment":           resourceEnvironment(),
			"phase_invite":                resourceInvite(),
			"phase_service_account":       resourceServiceAccount(),
			"phase_service_account_token": resourceServiceAccountToken(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"phase_secrets": dataSourceSecrets(),
			"phase_role":    dataSourceRole(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	host := strings.TrimRight(d.Get("host").(string), "/")
	if host == "" || host == DefaultHostURL {
		// api.phase.dev only fronts the public REST API; the SDK talks to the console host,
		// which serves both the end-to-end encrypted secrets endpoints and the REST API.
		host = misc.PhaseCloudAPIHost
	}

	// The SDK reads this process-wide flag when it first builds its HTTP client.
	misc.VerifySSL = !d.Get("skip_tls_verification").(bool)
	network.SetUserAgent(fmt.Sprintf("%s %s", UserAgent, sdkUserAgent))

	client, err := phase.New(d.Get("phase_token").(string), host, false)
	if err != nil {
		return nil, diag.Errorf("invalid phase_token: %s (expected a full pss_service:v2 or pss_user:v1 token)", err)
	}
	return client, nil
}

// isNotFound reports whether err is an HTTP 404 from the Phase API.
func isNotFound(err error) bool {
	var apiErr *network.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// isConflict reports whether err is an HTTP 409 from the Phase API.
func isConflict(err error) bool {
	var apiErr *network.APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusConflict
}

// stringList converts a Terraform list/set of strings to a Go slice (never nil).
func stringList(v interface{}) []string {
	out := []string{}
	switch list := v.(type) {
	case []interface{}:
		for _, item := range list {
			out = append(out, item.(string))
		}
	case *schema.Set:
		for _, item := range list.List() {
			out = append(out, item.(string))
		}
	}
	return out
}

// derefString returns the pointed-to string or "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// suppressTrimmed ignores leading/trailing whitespace differences, which the server strips.
func suppressTrimmed(k, old, new string, d *schema.ResourceData) bool {
	return strings.TrimSpace(old) == strings.TrimSpace(new)
}

// suppressEqualFold ignores case-only differences.
func suppressEqualFold(k, old, new string, d *schema.ResourceData) bool {
	return strings.EqualFold(old, new)
}

// suppressAfterCreate ignores changes to an argument that is only applied at creation.
func suppressAfterCreate(k, old, new string, d *schema.ResourceData) bool {
	return d.Id() != ""
}

// suppressSameInstant ignores formatting differences between timestamps denoting the same instant.
func suppressSameInstant(k, old, new string, d *schema.ResourceData) bool {
	o, errOld := time.Parse(time.RFC3339Nano, old)
	n, errNew := time.Parse(time.RFC3339Nano, new)
	return errOld == nil && errNew == nil && o.Equal(n)
}
