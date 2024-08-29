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
            "service_token": {
                Type:        schema.TypeString,
                Required:    true,
                Sensitive:   true,
                DefaultFunc: schema.EnvDefaultFunc("PHASE_SERVICE_TOKEN", nil),
                Description: "The service token for authenticating with Phase. Can be set with PHASE_SERVICE_TOKEN environment variable.",
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
    host := d.Get("host").(string)
    serviceToken := d.Get("service_token").(string)

    bearerToken := extractBearerToken(serviceToken)

    client := &PhaseClient{
        HostURL:    host,
        HTTPClient: &http.Client{},
        Token:      bearerToken,
    }

    return client, nil
}

func extractBearerToken(serviceToken string) string {
    parts := strings.Split(serviceToken, ":")
    if len(parts) >= 3 {
        return parts[2]
    }
    return ""
}

func resourceSecret() *schema.Resource {
    return &schema.Resource{
        CreateContext: resourceSecretCreate,
        ReadContext:   resourceSecretRead,
        UpdateContext: resourceSecretUpdate,
        DeleteContext: resourceSecretDelete,

        Schema: map[string]*schema.Schema{
            "application_id": {
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
            "tags": {
                Type:     schema.TypeSet,
                Optional: true,
                Elem:     &schema.Schema{Type: schema.TypeString},
            },
            "path": {
                Type:     schema.TypeString,
                Optional: true,
                Default:  "/",
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
        Tags:    expandStringSet(d.Get("tags").(*schema.Set)),
        Path:    d.Get("path").(string),
    }

    appID := d.Get("application_id").(string)
    env := d.Get("env").(string)

    createdSecret, err := client.CreateSecret(appID, env, secret)
    if err != nil {
        return diag.FromErr(err)
    }

    d.SetId(createdSecret.ID)
    return resourceSecretRead(ctx, d, meta)
}

func resourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
    client := meta.(*PhaseClient)

    appID := d.Get("application_id").(string)
    env := d.Get("env").(string)
    secretID := d.Id()

    secret, err := client.ReadSecret(appID, env, secretID)
    if err != nil {
        return diag.FromErr(err)
    }

    d.Set("key", secret.Key)
    d.Set("value", secret.Value)
    d.Set("comment", secret.Comment)
    d.Set("tags", secret.Tags)
    d.Set("path", secret.Path)

    return nil
}

func resourceSecretUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
    client := meta.(*PhaseClient)

    secret := Secret{
        ID:      d.Id(),
        Key:     d.Get("key").(string),
        Value:   d.Get("value").(string),
        Comment: d.Get("comment").(string),
        Tags:    expandStringSet(d.Get("tags").(*schema.Set)),
        Path:    d.Get("path").(string),
    }

    appID := d.Get("application_id").(string)
    env := d.Get("env").(string)

    _, err := client.UpdateSecret(appID, env, secret)
    if err != nil {
        return diag.FromErr(err)
    }

    return resourceSecretRead(ctx, d, meta)
}

func resourceSecretDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
    client := meta.(*PhaseClient)

    appID := d.Get("application_id").(string)
    env := d.Get("env").(string)
    secretID := d.Id()

    err := client.DeleteSecret(appID, env, secretID)
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
            "application_id": {
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
                        "tags": {
                            Type:     schema.TypeList,
                            Computed: true,
                            Elem:     &schema.Schema{Type: schema.TypeString},
                        },
                        "path": {
                            Type:     schema.TypeString,
                            Computed: true,
                        },
                    },
                },
            },
        },
    }
}

func dataSourceSecretsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
    client := meta.(*PhaseClient)

    appID := d.Get("application_id").(string)
    env := d.Get("env").(string)
    path := d.Get("path").(string)

    secrets, err := client.ListSecrets(appID, env, path)
    if err != nil {
        return diag.FromErr(err)
    }

    if err := d.Set("secrets", flattenSecrets(secrets)); err != nil {
        return diag.FromErr(err)
    }

    d.SetId(fmt.Sprintf("%s-%s-%s", appID, env, path))

    return nil
}

func expandStringSet(set *schema.Set) []string {
    list := set.List()
    result := make([]string, len(list))
    for i, v := range list {
        result[i] = v.(string)
    }
    return result
}

func flattenSecrets(secrets []Secret) []interface{} {
    var result []interface{}
    for _, secret := range secrets {
        s := map[string]interface{}{
            "id":      secret.ID,
            "key":     secret.Key,
            "value":   secret.Value,
            "comment": secret.Comment,
            "tags":    secret.Tags,
            "path":    secret.Path,
        }
        result = append(result, s)
    }
    return result
}