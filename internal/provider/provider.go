package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/terraform-provider/internal/client"
	"github.com/phasehq/terraform-provider/internal/config"
	"github.com/phasehq/terraform-provider/internal/datasource"
	"github.com/phasehq/terraform-provider/internal/resources"
)

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHASE_HOST", config.DefaultHostURL),
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
				Description: "Whether to skip SSL/TLS certificate validation for the PHASE_HOST. Defaults to false.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"phase_secret":      resources.Secret(),
			"phase_application": resources.Application(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"phase_secrets": datasource.Secrets(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
	host := d.Get("host").(string)
	phaseToken := d.Get("phase_token").(string)
	skipTLSVerification := d.Get("skip_tls_verification").(bool)
	client := client.NewPhaseClient(host, phaseToken, skipTLSVerification)
	return client, nil
}
