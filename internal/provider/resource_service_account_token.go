package provider

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/phasehq/golang-sdk/v2/phase"
)

func resourceServiceAccountToken() *schema.Resource {
	return &schema.Resource{
		Description:   "Creates an additional token for a service account. Tokens cannot be modified; changes force a new token.",
		CreateContext: resourceServiceAccountTokenCreate,
		ReadContext:   resourceServiceAccountTokenRead,
		DeleteContext: resourceServiceAccountTokenDelete,

		Schema: map[string]*schema.Schema{
			"service_account_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				DiffSuppressFunc: suppressTrimmed,
			},
			"expires_in": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"expires_at"},
				ValidateFunc:  validation.IntAtLeast(1),
				Description:   "Seconds until the token expires. Without expires_in or expires_at the token never expires.",
			},
			"expires_at": {
				Type:             schema.TypeString,
				Optional:         true,
				Computed:         true,
				ForceNew:         true,
				ConflictsWith:    []string{"expires_in"},
				DiffSuppressFunc: suppressSameInstant,
				Description:      "RFC 3339 expiry timestamp with a UTC offset, e.g. 2027-01-01T00:00:00Z.",
			},
			"token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The full token (pss_service:v2:...), only available at creation.",
			},
			"bearer_token": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The REST API bearer value (ServiceAccount <token>), only available at creation.",
			},
			"created_at": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func resourceServiceAccountTokenCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	token, err := client.CreateServiceAccountToken(d.Get("service_account_id").(string), phase.CreateServiceAccountTokenOptions{
		Name:      d.Get("name").(string),
		ExpiresAt: d.Get("expires_at").(string),
		ExpiresIn: d.Get("expires_in").(int),
	})
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(token.ID)
	d.Set("token", token.Token)
	d.Set("bearer_token", token.BearerToken)
	return resourceServiceAccountTokenRead(ctx, d, meta)
}

func resourceServiceAccountTokenRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	// Tokens are only listed on the service account detail.
	sa, err := client.GetServiceAccount(d.Get("service_account_id").(string))
	if err != nil {
		if isNotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}
	for _, token := range sa.Tokens {
		if token.ID == d.Id() {
			d.Set("name", token.Name)
			d.Set("expires_at", derefString(token.ExpiresAt))
			d.Set("created_at", token.CreatedAt)
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceServiceAccountTokenDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*phase.Phase)

	if err := client.DeleteServiceAccountToken(d.Get("service_account_id").(string), d.Id()); err != nil && !isNotFound(err) {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
