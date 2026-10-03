package provider

import (
	"context"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/phasehq/golang-sdk/v2/phase"
)

func resourceInvite() *schema.Resource {
	return &schema.Resource{
		Description: "Invites a user to the organisation by email. Once the invite is accepted the resource is kept " +
			"in state; if it expires or is cancelled outside Terraform it is re-created on the next apply. " +
			"Roles with global access, or that can create service account tokens, cannot be invited.",
		CreateContext: resourceInviteCreate,
		ReadContext:   resourceInviteRead,
		DeleteContext: resourceInviteDelete,

		Schema: map[string]*schema.Schema{
			"email": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				DiffSuppressFunc: suppressEqualFold, // Phase stores the address lower-cased
				Description:      "The email address to invite.",
			},
			"role_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the role to assign. Use the phase_role data source to look it up by name.",
			},
			"expires_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceInviteCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	invite, err := client.CreateInvite(phase.CreateInviteOptions{
		Email:  d.Get("email").(string),
		RoleID: d.Get("role_id").(string),
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(invite.ID)
	d.Set("expires_at", invite.ExpiresAt)
	d.Set("created_at", invite.CreatedAt)
	return nil
}

func resourceInviteRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	// There is no single-invite endpoint: pending invites are listed, accepted ones become members.
	invites, err := client.ListInvites()
	if err != nil {
		return diag.FromErr(err)
	}
	for _, invite := range invites {
		if invite.ID == d.Id() {
			d.Set("email", invite.InviteeEmail)
			if invite.Role != nil {
				d.Set("role_id", invite.Role.ID)
			}
			d.Set("expires_at", invite.ExpiresAt)
			d.Set("created_at", invite.CreatedAt)
			return nil
		}
	}

	members, err := client.ListMembers()
	if err != nil {
		return diag.FromErr(err)
	}
	for _, member := range members {
		if strings.EqualFold(member.Email, d.Get("email").(string)) {
			return nil // accepted: keep the resource as-is
		}
	}

	d.SetId("")
	return nil
}

func resourceInviteDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	if err := client.DeleteInvite(d.Id()); err != nil && !isNotFound(err) {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
