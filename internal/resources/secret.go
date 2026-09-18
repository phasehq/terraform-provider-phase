package resources

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/terraform-provider/internal/client"
	"github.com/phasehq/terraform-provider/internal/secrets"
)

func Secret() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSecretCreate,
		ReadContext:   resourceSecretRead,
		UpdateContext: resourceSecretUpdate,
		DeleteContext: resourceSecretDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceSecretImportState,
		},

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

func resourceSecretCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	secret := secrets.Secret{
		Key:     d.Get("key").(string),
		Value:   d.Get("value").(string),
		Comment: d.Get("comment").(string),
		Path:    d.Get("path").(string),
	}

	// Handle tags if present
	if v, ok := d.GetOk("tags"); ok {
		tags := make([]string, 0)
		for _, tag := range v.([]any) {
			tags = append(tags, tag.(string))
		}
		secret.Tags = tags
	}

	if v, ok := d.GetOk("override"); ok {
		overrideSet := v.(*schema.Set).List()
		if len(overrideSet) > 0 {
			overrideMap := overrideSet[0].(map[string]any)
			secret.Override = &secrets.SecretOverride{
				Value:    overrideMap["value"].(string),
				IsActive: overrideMap["is_active"].(bool),
			}
		}
	}

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)

	// First, try to create the secret - workaround for updating secrets via KEYs.
	createdSecret, err := secrets.CreateSecret(client, appID, env, secret)
	if err != nil {
		// If we get a 409 Conflict error, the secret already exists, so try to update it instead
		if strings.Contains(err.Error(), "409 Conflict") {
			// Try to read the existing secret first to get its ID
			existingSecrets, readErr := secrets.ReadSecret(client, appID, env, secret.Key)
			if readErr != nil {
				return diag.FromErr(fmt.Errorf("error reading existing secret: %w", readErr))
			}

			if len(existingSecrets) > 0 {
				// Set the ID from the existing secret
				secret.ID = existingSecrets[0].ID

				// Now attempt to update
				updatedSecret, updateErr := secrets.UpdateSecret(client, appID, env, secret)
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

func resourceSecretRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	secretKey := d.Get("key").(string)

	secrets, err := secrets.ReadSecret(client, appID, env, secretKey)
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
		d.Set("override", []any{
			map[string]any{
				"value":     secret.Override.Value,
				"is_active": secret.Override.IsActive,
			},
		})
	} else {
		d.Set("value", secret.Value)
		d.Set("override", []any{})
	}

	return nil
}

func resourceSecretUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	secret := secrets.Secret{
		ID:      d.Id(),
		Key:     d.Get("key").(string),
		Value:   d.Get("value").(string),
		Comment: d.Get("comment").(string),
		Path:    d.Get("path").(string),
	}

	// Handle tags if present
	if v, ok := d.GetOk("tags"); ok {
		tags := make([]string, 0)
		for _, tag := range v.([]any) {
			tags = append(tags, tag.(string))
		}
		secret.Tags = tags
	}

	if v, ok := d.GetOk("override"); ok {
		overrideSet := v.(*schema.Set).List()
		if len(overrideSet) > 0 {
			overrideMap := overrideSet[0].(map[string]any)
			secret.Override = &secrets.SecretOverride{
				Value:    overrideMap["value"].(string),
				IsActive: overrideMap["is_active"].(bool),
			}
		}
	}

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)

	_, err := secrets.UpdateSecret(client, appID, env, secret)
	if err != nil {
		return diag.FromErr(err)
	}

	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	secretID := d.Id()

	err := secrets.DeleteSecret(client, appID, env, secretID)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func resourceSecretImportState(ctx context.Context, d *schema.ResourceData, meta any) ([]*schema.ResourceData, error) {
	client := meta.(*client.PhaseClient)
	importID := d.Id()

	// Parse the secret imported secret {app_id}:{env}:{path}:{key} - 907549ca-1430-4aa0-9998-290525741005:production:/folder/path/:SECRET_1
	parts := strings.SplitN(importID, ":", 4)
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return nil, fmt.Errorf("unexpected format of ID (%s), expected {app_id}:{env}:{path}:{key}", importID)
	}

	appID := parts[0]
	env := parts[1]
	path := parts[2]
	key := parts[3]

	// Fetch all secrets at that given path
	secretsAtPath, err := secrets.ListSecrets(client, appID, env, path)
	if err != nil {
		// Handle API errors from ListSecrets
		return nil, fmt.Errorf("error listing secrets for path '%s' during import: %w", path, err)
	}

	// Find the specific secret matching the key within the results from the path
	var targetSecret *secrets.Secret
	for i := range secretsAtPath {
		if secretsAtPath[i].Key == key {
			targetSecret = &secretsAtPath[i]
			break
		}
	}

	// If no secret was found, return an error
	if targetSecret == nil {
		return nil, fmt.Errorf("no secret found with key '%s' at path '%s' in app '%s', env '%s'", key, path, appID, env)
	}

	// Populate the rest of the resource data
	d.Set("app_id", appID)
	d.Set("env", env)
	d.Set("key", key)
	d.SetId(targetSecret.ID)
	d.Set("path", targetSecret.Path)
	d.Set("comment", targetSecret.Comment)
	d.Set("tags", targetSecret.Tags)
	d.Set("version", targetSecret.Version)
	d.Set("created_at", targetSecret.CreatedAt)
	d.Set("updated_at", targetSecret.UpdatedAt)

	// Handle personal secret overrides
	if targetSecret.Override != nil && targetSecret.Override.IsActive {
		d.Set("value", targetSecret.Override.Value)
		// Ensure the override block in state reflects the imported override
		overrideState := []any{
			map[string]any{ // Convert SecretOverride struct to map[string]any
				"value":     targetSecret.Override.Value,
				"is_active": targetSecret.Override.IsActive,
			},
		}
		if err := d.Set("override", overrideState); err != nil {
			return nil, fmt.Errorf("error setting override state during import: %w", err)
		}
	} else {
		d.Set("value", targetSecret.Value)
		// Clear the override block if no active override exists
		if err := d.Set("override", []any{}); err != nil {
			return nil, fmt.Errorf("error clearing override state during import: %w", err)
		}
	}

	return []*schema.ResourceData{d}, nil
}
