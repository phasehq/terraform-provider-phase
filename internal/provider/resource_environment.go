package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/v2/phase"
)

func resourceEnvironment() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a custom environment within a Phase app. The app must use server-side encryption (SSE).",
		CreateContext: resourceEnvironmentCreate,
		ReadContext:   resourceEnvironmentRead,
		UpdateContext: resourceEnvironmentUpdate,
		DeleteContext: resourceEnvironmentDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceEnvironmentImportState,
		},

		Schema: map[string]*schema.Schema{
			"app_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the app the environment belongs to.",
			},
			"name": {
				Type:             schema.TypeString,
				Required:         true,
				DiffSuppressFunc: suppressTrimmed,
				Description:      "The name of the environment (letters, digits, - and _; max 64 characters).",
			},
			"env_type": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"index": {
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
		},
	}
}

func setEnvironmentState(d *schema.ResourceData, env *phase.Environment) {
	d.Set("name", env.Name)
	d.Set("env_type", env.EnvType)
	d.Set("index", env.Index)
	d.Set("created_at", env.CreatedAt)
	d.Set("updated_at", env.UpdatedAt)
}

func resourceEnvironmentCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	env, err := client.CreateEnvironment(d.Get("app_id").(string), d.Get("name").(string))
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(env.ID)
	return resourceEnvironmentRead(ctx, d, meta)
}

func resourceEnvironmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	env, err := client.GetEnvironment(d.Id())
	if err != nil {
		if isNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	setEnvironmentState(d, env)
	return nil
}

func resourceEnvironmentUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	if _, err := client.UpdateEnvironment(d.Id(), d.Get("name").(string)); err != nil {
		return diag.FromErr(err)
	}

	return resourceEnvironmentRead(ctx, d, meta)
}

func resourceEnvironmentDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	if err := client.DeleteEnvironment(d.Id()); err != nil && !isNotFound(err) {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func resourceEnvironmentImportState(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	// {app_id}:{environment_id}
	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("unexpected format of ID (%s), expected {app_id}:{environment_id}", d.Id())
	}
	d.Set("app_id", parts[0])
	d.SetId(parts[1])
	return []*schema.ResourceData{d}, nil
}
