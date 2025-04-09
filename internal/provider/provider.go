package provider

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

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
				Description: "Whether to skip SSL/TLS certificate validation for the PHASE_HOST. Defaults to false.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"phase_secret": resourceSecret(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"phase_secrets": dataSourceSecrets(),
		},
		ConfigureContextFunc: providerConfigure,
	}
}

func providerConfigure(ctx context.Context, d *schema.ResourceData) (interface{}, diag.Diagnostics) {
	phaseToken := d.Get("phase_token").(string)
	host := d.Get("host").(string)
	skipTLSVerification := d.Get("skip_tls_verification").(bool)

	if host != DefaultHostURL {
		host = fmt.Sprintf("%s/service/public", host)
	}

	tokenType, bearerToken := extractTokenInfo(phaseToken)

	client := &PhaseClient{
		HostURL:             host,
		HTTPClient:          &http.Client{},
		Token:               bearerToken,
		TokenType:           tokenType,
		SkipTLSVerification: skipTLSVerification,
	}

	return client, nil
}

func extractTokenInfo(phaseToken string) (string, string) {
	// First, check if it's a service token
	if PssServicePattern.MatchString(phaseToken) {
		parts := strings.Split(phaseToken, ":")
		if len(parts) >= 3 {
			version := parts[1]
			bearerToken := parts[2]

			// For service tokens with v2
			if version == "v2" {
				return "ServiceAccount", bearerToken
			}
			return "Service", bearerToken
		}
	}

	// Then check if it's a user token
	if PssUserPattern.MatchString(phaseToken) {
		parts := strings.Split(phaseToken, ":")
		if len(parts) >= 3 {
			return "User", parts[2]
		}
	}

	return "", phaseToken
}

func resourceSecret() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSecretCreate,
		ReadContext:   resourceSecretRead,
		UpdateContext: resourceSecretUpdate,
		DeleteContext: resourceSecretDelete,

		Schema: map[string]*schema.Schema{
			"app_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"env": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"key": {
				Type:     schema.TypeString,
				Required: true,
			},
			"value": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
			},
			"comment": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"path": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "/",
			},
			"tags": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"version": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"updated_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"override": {
				Type:     schema.TypeSet,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"value": {
							Type:      schema.TypeString,
							Required:  true,
							Sensitive: true,
						},
						"is_active": {
							Type:     schema.TypeBool,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceSecretCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*PhaseClient)

	secret := Secret{
		Key:     d.Get("key").(string),
		Value:   d.Get("value").(string),
		Comment: d.Get("comment").(string),
		Path:    d.Get("path").(string),
	}

	// Handle tags if present
	if v, ok := d.GetOk("tags"); ok {
		tags := make([]string, 0)
		for _, tag := range v.([]interface{}) {
			tags = append(tags, tag.(string))
		}
		secret.Tags = tags
	}

	if v, ok := d.GetOk("override"); ok {
		overrideSet := v.(*schema.Set).List()
		if len(overrideSet) > 0 {
			overrideMap := overrideSet[0].(map[string]interface{})
			secret.Override = &SecretOverride{
				Value:    overrideMap["value"].(string),
				IsActive: overrideMap["is_active"].(bool),
			}
		}
	}

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)

	// First, try to create the secret - workaround for updating secrets via KEYs.
	createdSecret, err := client.CreateSecret(appID, env, fmt.Sprintf("Bearer %s", client.TokenType), secret)
	if err != nil {
		// If we get a 409 Conflict error, the secret already exists, so try to update it instead
		if strings.Contains(err.Error(), "409 Conflict") {
			// Try to read the existing secret first to get its ID
			existingSecrets, readErr := client.ReadSecret(appID, env, secret.Key, fmt.Sprintf("Bearer %s", client.TokenType))
			if readErr != nil {
				return diag.FromErr(fmt.Errorf("error reading existing secret: %w", readErr))
			}

			if len(existingSecrets) > 0 {
				// Set the ID from the existing secret
				secret.ID = existingSecrets[0].ID

				// Now attempt to update
				updatedSecret, updateErr := client.UpdateSecret(appID, env, fmt.Sprintf("Bearer %s", client.TokenType), secret)
				if updateErr != nil {
					return diag.FromErr(fmt.Errorf("error updating existing secret: %w", updateErr))
				}

				d.SetId(updatedSecret.ID)
				return resourceSecretRead(ctx, d, meta)
			} else {
				return diag.FromErr(fmt.Errorf("received 409 Conflict but couldn't find existing secret: %w", err))
			}
		}
		return diag.FromErr(err)
	}

	d.SetId(createdSecret.ID)
	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*PhaseClient)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	secretKey := d.Get("key").(string)

	secrets, err := client.ReadSecret(appID, env, secretKey, fmt.Sprintf("Bearer %s", client.TokenType))
	if err != nil {
		return diag.FromErr(err)
	}

	if len(secrets) == 0 {
		return diag.Errorf("No secrets found")
	}

	// If a specific key was provided, use the first (and should be only) secret
	secret := secrets[0]

	d.SetId(secret.ID)
	d.Set("key", secret.Key)
	d.Set("comment", secret.Comment)
	d.Set("path", secret.Path)

	// Set the new fields
	if secret.Tags != nil {
		d.Set("tags", secret.Tags)
	}
	d.Set("version", secret.Version)
	d.Set("created_at", secret.CreatedAt)
	d.Set("updated_at", secret.UpdatedAt)

	if secret.Override != nil && secret.Override.IsActive {
		d.Set("value", secret.Override.Value)
		d.Set("override", []interface{}{
			map[string]interface{}{
				"value":     secret.Override.Value,
				"is_active": secret.Override.IsActive,
			},
		})
	} else {
		d.Set("value", secret.Value)
		d.Set("override", []interface{}{})
	}

	return nil
}

func resourceSecretUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*PhaseClient)

	secret := Secret{
		ID:      d.Id(),
		Key:     d.Get("key").(string),
		Value:   d.Get("value").(string),
		Comment: d.Get("comment").(string),
		Path:    d.Get("path").(string),
	}

	// Handle tags if present
	if v, ok := d.GetOk("tags"); ok {
		tags := make([]string, 0)
		for _, tag := range v.([]interface{}) {
			tags = append(tags, tag.(string))
		}
		secret.Tags = tags
	}

	if v, ok := d.GetOk("override"); ok {
		overrideSet := v.(*schema.Set).List()
		if len(overrideSet) > 0 {
			overrideMap := overrideSet[0].(map[string]interface{})
			secret.Override = &SecretOverride{
				Value:    overrideMap["value"].(string),
				IsActive: overrideMap["is_active"].(bool),
			}
		}
	}

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)

	_, err := client.UpdateSecret(appID, env, fmt.Sprintf("Bearer %s", client.TokenType), secret)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*PhaseClient)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	secretID := d.Id()

	err := client.DeleteSecret(appID, env, secretID, fmt.Sprintf("Bearer %s", client.TokenType))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func dataSourceSecrets() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceSecretsRead,
		Schema: map[string]*schema.Schema{
			"app_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the Phase App.",
			},
			"env": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The environment name.",
			},
			"path": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "/",
				Description: "The path to fetch secrets from.",
			},
			"key": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The key of a specific secret to fetch.",
			},
			"tags": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of tags to filter secrets by. Multiple tags are combined with OR logic.",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"secrets": {
				Type:      schema.TypeMap,
				Computed:  true,
				Sensitive: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func dataSourceSecretsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*PhaseClient)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	path := d.Get("path").(string)
	key := d.Get("key").(string)

	// Handle tags if present
	var tagsFilter string
	if v, ok := d.GetOk("tags"); ok {
		tags := make([]string, 0)
		for _, tag := range v.([]interface{}) {
			tags = append(tags, tag.(string))
		}
		if len(tags) > 0 {
			tagsFilter = strings.Join(tags, ",")
		}
	}

	// Determine if we're fetching all secrets
	fetchingAll := path == ""

	secrets, err := client.ReadSecret(appID, env, key, fmt.Sprintf("Bearer %s", client.TokenType), tagsFilter)
	if err != nil {
		return diag.FromErr(err)
	}

	secretMap := make(map[string]string)
	for _, secret := range secrets {
		if fetchingAll || secret.Path == path {
			if secret.Override != nil && secret.Override.IsActive {
				secretMap[secret.Key] = secret.Override.Value
			} else {
				secretMap[secret.Key] = secret.Value
			}
		}
	}

	if err := d.Set("secrets", secretMap); err != nil {
		return diag.FromErr(err)
	}

	// Set the path in the state
	if err := d.Set("path", path); err != nil {
		return diag.FromErr(err)
	}

	// Generate a unique ID for the data source
	d.SetId(fmt.Sprintf("%s-%s-%s-%s-%s", appID, env, path, key, tagsFilter))

	return nil
}
