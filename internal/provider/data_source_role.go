package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/v2/phase"
)

func dataSourceRole() *schema.Resource {
	return &schema.Resource{
		Description: "Looks up an organisation role by name, e.g. to reference its ID from phase_invite or phase_service_account.",
		ReadContext: dataSourceRoleRead,
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The role name (case-insensitive), e.g. Developer or Service.",
			},
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"is_default": {
				Type:     schema.TypeBool,
				Computed: true,
			},
		},
	}
}

func dataSourceRoleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	roles, err := client.ListRoles()
	if err != nil {
		return diag.FromErr(err)
	}

	name := d.Get("name").(string)
	for _, role := range roles {
		if strings.EqualFold(role.Name, name) {
			d.SetId(role.ID)
			d.Set("name", role.Name)
			d.Set("description", role.Description)
			d.Set("is_default", role.IsDefault)
			return nil
		}
	}

	return diag.Errorf("no role named %q found in the organisation", name)
}
