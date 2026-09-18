package resources

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/terraform-provider/internal/applications"
	"github.com/phasehq/terraform-provider/internal/client"
)

func Application() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceApplicationCreate,
		ReadContext:   resourceApplicationRead,
		UpdateContext: resourceApplicationUpdate,
		DeleteContext: resourceApplicationDelete,
		Importer: &schema.ResourceImporter{
			StateContext: resourceApplicationImportState,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Optional: true,
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

func resourceApplicationCreate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	app := applications.CreateApplicationRequest{
		Name: d.Get("name").(string),
	}

	if v, ok := d.GetOk("description"); ok {
		description := v.(string)
		app.Description = &description
	}

	createdApplication, err := applications.CreateApplication(client, app)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(createdApplication.ID)
	d.Set("name", createdApplication.Name)
	if createdApplication.Description != "" {
		d.Set("description", createdApplication.Description)
	}
	d.Set("created_at", createdApplication.CreatedAt)
	d.Set("updated_at", createdApplication.UpdatedAt)

	return nil
}

func resourceApplicationRead(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	application, err := applications.ReadApplication(client, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if application == nil {
		return diag.Errorf("No app found")
	}

	d.SetId(application.ID)
	d.Set("name", application.Name)
	if application.Description != "" {
		d.Set("description", application.Description)
	}
	d.Set("created_at", application.CreatedAt)
	d.Set("updated_at", application.UpdatedAt)

	return nil
}

func resourceApplicationUpdate(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	app := applications.Application{
		ID:   d.Id(),
		Name: d.Get("name").(string),
	}

	// Handle description if present
	if v, ok := d.GetOk("description"); ok {
		app.Description = v.(string)
	}

	application, err := applications.UpdateApplication(client, d.Id(), app)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(application.ID)
	d.Set("name", application.Name)
	if application.Description != "" {
		d.Set("description", application.Description)
	}
	d.Set("created_at", application.CreatedAt)
	d.Set("updated_at", application.UpdatedAt)

	return nil
}

func resourceApplicationDelete(ctx context.Context, d *schema.ResourceData, meta any) diag.Diagnostics {
	client := meta.(*client.PhaseClient)

	err := applications.DeleteApplication(client, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

func resourceApplicationImportState(
	ctx context.Context,
	d *schema.ResourceData,
	meta any,
) ([]*schema.ResourceData, error) {
	applicationID := d.Id()

	if applicationID == "" {
		return nil, fmt.Errorf("application ID cannot be empty")
	}

	return []*schema.ResourceData{d}, nil
}
