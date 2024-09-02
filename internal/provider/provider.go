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

	if host != DefaultHostURL {
		host = fmt.Sprintf("%s/service/public", host)
	}

	tokenType, bearerToken := extractTokenInfo(phaseToken)

	client := &PhaseClient{
		HostURL:    host,
		HTTPClient: &http.Client{},
		Token:      bearerToken,
		TokenType:  tokenType,
	}

	return client, nil
}

func extractTokenInfo(phaseToken string) (string, string) {
	if PssUserPattern.MatchString(phaseToken) {
		parts := strings.Split(phaseToken, ":")
		if len(parts) >= 3 {
			return "User", parts[2]
		}
	} else if PssServicePattern.MatchString(phaseToken) {
		parts := strings.Split(phaseToken, ":")
		if len(parts) >= 3 {
			return "Service", parts[2]
		}
	}
	return "", phaseToken // Default to empty token type if no match
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

	createdSecret, err := client.CreateSecret(appID, env, fmt.Sprintf("Bearer %s", client.TokenType), secret)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(createdSecret.ID)
	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*PhaseClient)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	secretID := d.Id()

	secret, err := client.ReadSecret(appID, env, secretID, fmt.Sprintf("Bearer %s", client.TokenType))
	if err != nil {
		return diag.FromErr(err)
	}

	d.Set("key", secret.Key)
	d.Set("comment", secret.Comment)
	d.Set("path", secret.Path)

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
			"secrets": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"key": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"value": {
							Type:      schema.TypeString,
							Computed:  true,
							Sensitive: true,
						},
						"comment": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"path": {
							Type:     schema.TypeString,
							Computed: true,
						},
						"override": {
							Type:     schema.TypeList,
							Computed: true,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"value": {
										Type:      schema.TypeString,
										Computed:  true,
										Sensitive: true,
									},
									"is_active": {
										Type:     schema.TypeBool,
										Computed: true,
									},
								},
							},
						},
					},
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

	secrets, err := client.ListSecrets(appID, env, path, fmt.Sprintf("Bearer %s", client.TokenType))
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("secrets", flattenSecrets(secrets)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s-%s-%s", appID, env, path))

	return nil
}

func flattenSecrets(secrets []Secret) []interface{} {
	var result []interface{}
	for _, secret := range secrets {
		s := map[string]interface{}{
			"id":      secret.ID,
			"key":     secret.Key,
			"comment": secret.Comment,
			"path":    secret.Path,
		}

		if secret.Override != nil && secret.Override.IsActive {
			s["value"] = secret.Override.Value
			s["override"] = []interface{}{
				map[string]interface{}{
					"value":     secret.Override.Value,
					"is_active": secret.Override.IsActive,
				},
			}
		} else {
			s["value"] = secret.Value
			s["override"] = []interface{}{}
		}

		result = append(result, s)
	}
	return result
}