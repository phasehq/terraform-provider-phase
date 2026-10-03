package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/v2/phase"
)

func resourceServiceAccount() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a service account with server-side key management, its initial token and its app access.",
		CreateContext: resourceServiceAccountCreate,
		ReadContext:   resourceServiceAccountRead,
		UpdateContext: resourceServiceAccountUpdate,
		DeleteContext: resourceServiceAccountDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:             schema.TypeString,
				Required:         true,
				DiffSuppressFunc: suppressTrimmed,
				Description:      "The name of the service account.",
			},
			"role_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The ID of the role to assign. Use the phase_role data source to look it up by name.",
			},
			"token_name": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				DiffSuppressFunc: suppressAfterCreate,
				Description:      "The name of the token created with the service account. Defaults to Default. Only applied at creation and ignored afterwards.",
			},
			"access": {
				Type:     schema.TypeSet,
				Optional: true,
				Computed: true,
				Description: "Apps and environments the service account can access. Declarative: apps not listed " +
					"lose access when the block is set. Only server-side encrypted apps can be granted.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"app_id": {
							Type:     schema.TypeString,
							Required: true,
						},
						"environments": {
							Type:        schema.TypeSet,
							Required:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Environment names within the app, as shown in the Console (e.g. Development).",
						},
					},
				},
			},
			"token_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The full initial token (pss_service:v2:...), only available at creation.",
			},
			"bearer_token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The initial token's REST API bearer value (ServiceAccount <token>), only available at creation.",
			},
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"updated_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

// accessFromResource resolves the configured access blocks to app and environment IDs.
func accessFromResource(client *phase.Phase, d *schema.ResourceData) ([]phase.AppAccessInput, error) {
	var access []phase.AppAccessInput
	for _, item := range d.Get("access").(*schema.Set).List() {
		block := item.(map[string]interface{})
		appID := block["app_id"].(string)

		envs, err := client.ListEnvironments(appID)
		if err != nil {
			return nil, fmt.Errorf("error listing environments of app %s: %w", appID, err)
		}
		var envIDs []string
		for _, name := range stringList(block["environments"]) {
			found := false
			for _, env := range envs {
				if strings.EqualFold(env.Name, name) {
					envIDs = append(envIDs, env.ID)
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("environment %q not found in app %s", name, appID)
			}
		}
		access = append(access, phase.AppAccessInput{ID: appID, EnvironmentIDs: envIDs})
	}
	return access, nil
}

// accessToState converts the server's view of app access into `access` blocks. Environment
// names keep the spelling used in the configuration when they only differ in case, and apps
// without server-side encryption (which the access API cannot manage) are left out.
func accessToState(d *schema.ResourceData, apps []phase.AppAccess, e2eeApps map[string]bool) []interface{} {
	configured := map[string]string{} // app_id + lower(name) -> configured name
	for _, item := range d.Get("access").(*schema.Set).List() {
		block := item.(map[string]interface{})
		for _, name := range stringList(block["environments"]) {
			configured[block["app_id"].(string)+"\x00"+strings.ToLower(name)] = name
		}
	}

	state := make([]interface{}, 0, len(apps))
	for _, app := range apps {
		if e2eeApps[app.ID] {
			continue
		}
		names := make([]interface{}, 0, len(app.Environments))
		for _, env := range app.Environments {
			name := env.Name
			if c, ok := configured[app.ID+"\x00"+strings.ToLower(name)]; ok {
				name = c
			}
			names = append(names, name)
		}
		state = append(state, map[string]interface{}{"app_id": app.ID, "environments": names})
	}
	return state
}

func setServiceAccountState(client *phase.Phase, d *schema.ResourceData, sa *phase.ServiceAccount) diag.Diagnostics {
	d.Set("name", sa.Name)
	if sa.Role != nil {
		d.Set("role_id", sa.Role.ID)
	}
	d.Set("created_at", sa.CreatedAt)
	d.Set("updated_at", sa.UpdatedAt)

	e2eeApps := map[string]bool{}
	if len(sa.Apps) > 0 {
		apps, err := client.ListApps()
		if err != nil {
			return diag.FromErr(err)
		}
		for _, app := range apps {
			if !app.SSEEnabled {
				e2eeApps[app.ID] = true
			}
		}
	}
	if err := d.Set("access", accessToState(d, sa.Apps, e2eeApps)); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceServiceAccountCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	sa, err := client.CreateServiceAccount(phase.CreateServiceAccountOptions{
		Name:      d.Get("name").(string),
		RoleID:    d.Get("role_id").(string),
		TokenName: d.Get("token_name").(string),
	})
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(sa.ID)

	if sa.InitialToken != nil {
		d.Set("token_id", sa.InitialToken.ID)
		d.Set("token_name", sa.InitialToken.Name)
		d.Set("token", sa.InitialToken.Token)
		d.Set("bearer_token", sa.InitialToken.BearerToken)
	}

	if _, ok := d.GetOk("access"); ok {
		access, err := accessFromResource(client, d)
		if err != nil {
			return diag.FromErr(err)
		}
		if _, err := client.SetServiceAccountAccess(sa.ID, access); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceServiceAccountRead(ctx, d, meta)
}

func resourceServiceAccountRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	sa, err := client.GetServiceAccount(d.Id())
	if err != nil {
		if isNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	return setServiceAccountState(client, d, sa)
}

func resourceServiceAccountUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	if d.HasChanges("name", "role_id") {
		opts := phase.UpdateServiceAccountOptions{}
		if d.HasChange("name") {
			opts.Name = d.Get("name").(string)
		}
		if d.HasChange("role_id") {
			opts.RoleID = d.Get("role_id").(string)
		}
		if _, err := client.UpdateServiceAccount(d.Id(), opts); err != nil {
			return diag.FromErr(err)
		}
	}

	if d.HasChange("access") {
		access, err := accessFromResource(client, d)
		if err != nil {
			return diag.FromErr(err)
		}
		if _, err := client.SetServiceAccountAccess(d.Id(), access); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourceServiceAccountRead(ctx, d, meta)
}

func resourceServiceAccountDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	if err := client.DeleteServiceAccount(d.Id()); err != nil && !isNotFound(err) {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
