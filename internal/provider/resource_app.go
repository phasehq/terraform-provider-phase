package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/v2/phase"
)

func resourceApp() *schema.Resource {
	return &schema.Resource{
		Description:   "Manages a Phase application. Apps created through the API use server-side encryption (SSE). Destroying an app permanently deletes its environments and secrets.",
		CreateContext: resourceAppCreate,
		ReadContext:   resourceAppRead,
		UpdateContext: resourceAppUpdate,
		DeleteContext: resourceAppDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:             schema.TypeString,
				Required:         true,
				DiffSuppressFunc: suppressTrimmed,
				Description:      "The name of the app.",
			},
			"description": {
				Type:             schema.TypeString,
				Optional:         true,
				DiffSuppressFunc: suppressTrimmed,
				Description:      "A description of the app.",
			},
			"environments": {
				Type:             schema.TypeList,
				Optional:         true,
				DiffSuppressFunc: suppressAfterCreate,
				Elem:             &schema.Schema{Type: schema.TypeString},
				Description:      "Names of custom environments to create with the app (paid plans). Defaults to Development, Staging and Production. Only applied at creation and ignored afterwards; use phase_environment to manage environments.",
			},
			"sse_enabled": {
				Type:     schema.TypeBool,
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

func setAppState(d *schema.ResourceData, app *phase.App) {
	d.Set("name", app.Name)
	d.Set("description", app.Description)
	d.Set("sse_enabled", app.SSEEnabled)
	d.Set("created_at", app.CreatedAt)
	d.Set("updated_at", app.UpdatedAt)
}

func resourceAppCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	app, err := client.CreateApp(phase.CreateAppOptions{
		Name:         d.Get("name").(string),
		Description:  d.Get("description").(string),
		Environments: stringList(d.Get("environments")),
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(app.ID)
	return resourceAppRead(ctx, d, meta)
}

func resourceAppRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	app, err := client.GetApp(d.Id())
	if err != nil {
		if isNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	setAppState(d, app)
	return nil
}

func resourceAppUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	opts := phase.UpdateAppOptions{}
	if d.HasChange("name") {
		opts.Name = d.Get("name").(string)
	}
	if d.HasChange("description") {
		description := d.Get("description").(string)
		opts.Description = &description
	}
	if _, err := client.UpdateApp(d.Id(), opts); err != nil {
		return diag.FromErr(err)
	}

	return resourceAppRead(ctx, d, meta)
}

func resourceAppDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	if err := client.DeleteApp(d.Id()); err != nil && !isNotFound(err) {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
