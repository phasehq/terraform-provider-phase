package provider

import (
	"context"
	"fmt"
	"runtime"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/phase"
	"github.com/phasehq/golang-sdk/phase/misc"
	"github.com/phasehq/golang-sdk/phase/network"
)

// Version of the provider
const Version = "0.1.0"

func Provider() *schema.Provider {
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"host": {
				Type:        schema.TypeString,
				Optional:    true,
				DefaultFunc: schema.EnvDefaultFunc("PHASE_HOST", "https://console.phase.dev"),
				Description: "The host URL for the Phase API. Can be set with PHASE_HOST environment variable. Defaults to Phase Cloud - https://console.phase.dev if not set.",
			},
			"service_token": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				DefaultFunc: schema.EnvDefaultFunc("PHASE_SERVICE_TOKEN", nil),
				Description: "The service token for authenticating with Phase. Can be set with PHASE_SERVICE_TOKEN environment variable.",
			},
		},
		DataSourcesMap: map[string]*schema.Resource{
			"phase_secrets": dataSourceSecrets(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	host := d.Get("host").(string)
	serviceToken := d.Get("service_token").(string)

	// Set the user agent for the SDK
	userAgent := fmt.Sprintf("terraform-provider-phase/%s phase-golang-sdk/%s (%s/%s)",
		Version,
		misc.Version,
		runtime.GOOS,
		runtime.GOARCH)
	network.SetUserAgent(userAgent)

	// Initialize the Phase client
	client := phase.Init(serviceToken, host, false)
	if client == nil {
		return nil, diag.Errorf("Failed to initialize Phase client")
	}

	return client, nil
}

func dataSourceSecrets() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceSecretsRead,
		Schema: map[string]*schema.Schema{
			"env": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The environment name.",
			},
			"application_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The application ID. This takes precedence over application_name if both are provided.",
			},
			"application_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The application name. Only used if application_id is not provided.",
			},
			"path": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "/",
				Description: "The path to fetch secrets from.",
			},
			"secrets": {
				Type:        schema.TypeMap,
				Computed:    true,
				Sensitive:   true,
				Description: "The map of fetched secrets.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func dataSourceSecretsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	env := d.Get("env").(string)
	appID := d.Get("application_id").(string)
	appName := d.Get("application_name").(string)
	path := d.Get("path").(string)

	opts := phase.GetAllSecretsOptions{
		EnvName:    env,
		AppID:      appID,
		AppName:    appName,
		SecretPath: path,
	}

	secrets, err := client.GetAll(opts)
	if err != nil {
		return diag.FromErr(err)
	}

	secretMap := make(map[string]string)
	for _, secret := range secrets {
		key, ok := secret["key"].(string)
		if !ok {
			continue
		}
		value, ok := secret["value"].(string)
		if !ok {
			continue
		}
		secretMap[key] = value
	}

	if err := d.Set("secrets", secretMap); err != nil {
		return diag.FromErr(err)
	}

	// Generate a unique ID for this data source
	identifier := appID
	if identifier == "" {
		identifier = appName
	}
	d.SetId(env + "-" + identifier + "-" + path)

	return nil
}