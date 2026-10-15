package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/v2/phase"
)

// Secrets are encrypted and decrypted client-side by the SDK: the keys embedded in the
// provider token unwrap the environment key, so the server never sees plaintext.

// referenceResolutionMu serialises reads that resolve ${...} references: the SDK keeps a
// process-wide reference cache that concurrent data source reads would otherwise race on.
var referenceResolutionMu sync.Mutex

func resourceSecret() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSecretCreate,
		ReadContext:   resourceSecretRead,
		UpdateContext: resourceSecretUpdate,
		DeleteContext: resourceSecretDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceSecretImportState,
		},
		// The server keeps the value of a sealed secret; changing it means re-creating the secret.
		CustomizeDiff: func(ctx context.Context, d *schema.ResourceDiff, meta interface{}) error {
			if d.Id() != "" && d.HasChange("value") {
				oldType, newType := d.GetChange("type")
				if oldType == phase.SecretTypeSealed || newType == phase.SecretTypeSealed {
					return d.ForceNew("value")
				}
			}
			return nil
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
			// A set: the API returns tags in no fixed order.
			"tags": {
				Type:     schema.TypeSet,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"type": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The secret type: secret (default), config or sealed. Sealed secrets cannot be unsealed and their value can only be changed by re-creating them.",
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

// overrideValue returns the configured personal override value, or "" when none is set.
func overrideValue(d *schema.ResourceData) string {
	if v, ok := d.GetOk("override"); ok {
		if items := v.(*schema.Set).List(); len(items) > 0 {
			return items[0].(map[string]interface{})["value"].(string)
		}
	}
	return ""
}

// checkEnvironment guards against the SDK's partial environment-name matching: a secret
// that came back from a different environment than configured is an error, not drift.
func checkEnvironment(secret *phase.SecretResult, env string) error {
	if !strings.EqualFold(secret.Environment, env) {
		return fmt.Errorf("environment %q resolved to %q; use the exact environment name", env, secret.Environment)
	}
	return nil
}

// findSecretByKey looks up a single secret by key within a path, without resolving references.
func findSecretByKey(client *phase.Phase, appID, env, path, key string) (*phase.SecretResult, error) {
	secrets, err := client.Get(phase.GetOptions{AppID: appID, EnvName: env, Path: path, Keys: []string{key}, Raw: true})
	if err != nil {
		return nil, err
	}
	for i := range secrets {
		if secrets[i].Key == key && !secrets[i].IsDynamic {
			if err := checkEnvironment(&secrets[i], env); err != nil {
				return nil, err
			}
			return &secrets[i], nil
		}
	}
	return nil, nil
}

// findSecretByID looks up a secret by its ID. The key lookup is tried first because the
// server filters it by digest (one secret, one decryption); the wider scans only run when
// the secret was renamed or moved outside of Terraform.
func findSecretByID(client *phase.Phase, appID, env, path, key, id string) (*phase.SecretResult, error) {
	if key != "" {
		secret, err := findSecretByKey(client, appID, env, path, key)
		if err != nil {
			return nil, err
		}
		if secret != nil && secret.ID == id {
			return secret, nil
		}
	}
	for _, p := range []string{path, ""} {
		secrets, err := client.Get(phase.GetOptions{AppID: appID, EnvName: env, Path: p, Raw: true})
		if err != nil {
			return nil, err
		}
		for i := range secrets {
			if secrets[i].ID == id {
				if err := checkEnvironment(&secrets[i], env); err != nil {
					return nil, err
				}
				return &secrets[i], nil
			}
		}
		if p == "" {
			break
		}
	}
	return nil, nil
}

func setSecretState(d *schema.ResourceData, secret *phase.SecretResult) diag.Diagnostics {
	d.SetId(secret.ID)
	d.Set("key", secret.Key)
	d.Set("comment", secret.Comment)
	d.Set("path", secret.Path)
	d.Set("tags", secret.Tags)
	d.Set("type", secret.Type)
	d.Set("version", secret.Version)
	d.Set("created_at", secret.CreatedAt)
	d.Set("updated_at", secret.UpdatedAt)

	if secret.Overridden {
		// The SDK returns the personal override in place of the shared value. Keep the
		// configured value for `value` so an active override is not reported as drift;
		// on import there is nothing configured yet, so record the override value.
		if d.Get("value").(string) == "" {
			d.Set("value", secret.Value)
		}
		if err := d.Set("override", []interface{}{map[string]interface{}{"value": secret.Value, "is_active": true}}); err != nil {
			return diag.FromErr(err)
		}
		return nil
	}

	d.Set("value", secret.Value)
	if err := d.Set("override", []interface{}{}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

// pushSecret writes the configured state of the secret identified by d.Id(), which currently
// lives at sourcePath. The key is only re-sent when setKey is true. When adopting a secret
// that already existed, only the arguments actually set in the configuration are written.
func pushSecret(client *phase.Phase, d *schema.ResourceData, sourcePath string, setKey, adopt bool) error {
	comment := d.Get("comment").(string)
	opts := phase.UpdateOptions{
		AppID:           d.Get("app_id").(string),
		EnvName:         d.Get("env").(string),
		ID:              d.Id(),
		SourcePath:      sourcePath,
		Value:           d.Get("value").(string),
		AllowEmptyValue: true,
		Type:            d.Get("type").(string),
		Comment:         &comment,
		Tags:            stringList(d.Get("tags")),
	}
	if adopt {
		if _, ok := d.GetOk("comment"); !ok {
			opts.Comment = nil
		}
		if _, ok := d.GetOk("tags"); !ok {
			opts.Tags = nil
		}
	}
	if setKey {
		opts.Key = d.Get("key").(string)
	}
	if path := d.Get("path").(string); path != sourcePath {
		opts.DestinationPath = path
	}
	if err := client.Update(opts); err != nil {
		return err
	}

	if ov := overrideValue(d); ov != "" {
		return client.Update(phase.UpdateOptions{
			AppID:      opts.AppID,
			EnvName:    opts.EnvName,
			ID:         d.Id(),
			SourcePath: d.Get("path").(string),
			Override:   true,
			Value:      ov,
		})
	}
	return nil
}

func resourceSecretCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	path := d.Get("path").(string)
	key := d.Get("key").(string)

	err := client.Create(phase.CreateOptions{
		AppID:         appID,
		EnvName:       env,
		Path:          path,
		OverrideValue: overrideValue(d),
		KeyValuePairs: []phase.KeyValuePair{{
			Key:     key,
			Value:   d.Get("value").(string),
			Comment: d.Get("comment").(string),
			Tags:    stringList(d.Get("tags")),
			Type:    d.Get("type").(string),
		}},
	})
	if err != nil && !isConflict(err) {
		return diag.FromErr(err)
	}

	secret, findErr := findSecretByKey(client, appID, env, path, key)
	if findErr != nil {
		return diag.FromErr(fmt.Errorf("error reading secret %q after create: %w", key, findErr))
	}
	if secret == nil {
		return diag.Errorf("secret %q was not found at path %q after creation", key, path)
	}
	d.SetId(secret.ID)

	if err != nil {
		// The secret already existed at this path: manage it in place rather than failing.
		if err := pushSecret(client, d, path, true, true); err != nil {
			return diag.FromErr(fmt.Errorf("error updating existing secret: %w", err))
		}
	}

	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	secret, err := findSecretByID(client, d.Get("app_id").(string), d.Get("env").(string), d.Get("path").(string), d.Get("key").(string), d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if secret == nil {
		d.SetId("")
		return nil
	}
	return setSecretState(d, secret)
}

func resourceSecretUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	oldPath, _ := d.GetChange("path")
	if err := pushSecret(client, d, oldPath.(string), d.HasChange("key"), false); err != nil {
		return diag.FromErr(err)
	}

	return resourceSecretRead(ctx, d, meta)
}

func resourceSecretDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	// Delete by the ID in state; the server ignores IDs that no longer exist.
	_, err := client.Delete(phase.DeleteOptions{
		AppID:   d.Get("app_id").(string),
		EnvName: d.Get("env").(string),
		IDs:     []string{d.Id()},
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func resourceSecretImportState(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	client := meta.(*phase.Phase)
	importID := d.Id()

	// Parse the secret imported secret {app_id}:{env}:{path}:{key} - 907549ca-1430-4aa0-9998-290525741005:production:/folder/path/:SECRET_1
	parts := strings.SplitN(importID, ":", 4)
	if len(parts) != 4 || parts[0] == "" || parts[1] == "" || parts[2] == "" || parts[3] == "" {
		return nil, fmt.Errorf("unexpected format of ID (%s), expected {app_id}:{env}:{path}:{key}", importID)
	}

	appID := parts[0]
	env := parts[1]
	path := strings.TrimSuffix(parts[2], "/")
	if path == "" {
		path = "/"
	}
	key := parts[3]

	secret, err := findSecretByKey(client, appID, env, path, key)
	if err != nil {
		return nil, fmt.Errorf("error listing secrets for path '%s' during import: %w", path, err)
	}
	if secret == nil {
		return nil, fmt.Errorf("no secret found with key '%s' at path '%s' in app '%s', env '%s'", key, path, appID, env)
	}

	d.Set("app_id", appID)
	d.Set("env", env)
	if diags := setSecretState(d, secret); diags.HasError() {
		return nil, fmt.Errorf("error setting state during import: %s", diags[0].Summary)
	}

	return []*schema.ResourceData{d}, nil
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
				Description: "The path to fetch secrets from. An empty string fetches secrets from all paths.",
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
	client := meta.(*phase.Phase)

	appID := d.Get("app_id").(string)
	env := d.Get("env").(string)
	path := d.Get("path").(string)
	key := d.Get("key").(string)
	tags := stringList(d.Get("tags"))

	opts := phase.GetOptions{AppID: appID, EnvName: env, Path: path, Tags: tags}
	if key != "" {
		opts.Keys = []string{key}
	}

	// References such as ${OTHER_KEY} are resolved by the SDK, as the REST API did server-side.
	referenceResolutionMu.Lock()
	secrets, err := client.Get(opts)
	referenceResolutionMu.Unlock()
	if err != nil {
		return diag.FromErr(err)
	}

	// No match is an empty map, not an error: a filter that selects nothing is a valid read.
	secretMap := make(map[string]string)
	for i := range secrets {
		if err := checkEnvironment(&secrets[i], env); err != nil {
			return diag.FromErr(err)
		}
		secretMap[secrets[i].Key] = secrets[i].Value
	}

	if err := d.Set("secrets", secretMap); err != nil {
		return diag.FromErr(err)
	}

	// Set the path in the state
	if err := d.Set("path", path); err != nil {
		return diag.FromErr(err)
	}

	// Generate a unique ID for the data source
	d.SetId(fmt.Sprintf("%s-%s-%s-%s-%s", appID, env, path, key, strings.Join(tags, ",")))

	return nil
}
