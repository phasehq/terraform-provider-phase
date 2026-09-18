package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var testAccProviders map[string]*schema.Provider
var testAccProvider *schema.Provider

func init() {
	testAccProvider = Provider()
	testAccProviders = map[string]*schema.Provider{
		"phase": testAccProvider,
	}
}

// TestProvider validates the provider's schema is internally consistent.
// This alone won't catch the resourceApplication schema gap unless its
// CreateContext is exercised, but it's a fast first check.
func TestProvider(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("provider InternalValidate failed: %s", err)
	}
}

// testAccPreCheck ensures required env vars are set before any TF_ACC test
// runs. resource.Test itself skips the whole test if TF_ACC is unset, so
// these acceptance tests never run in a normal `go test ./...`.
func testAccPreCheck(t *testing.T) {
	if os.Getenv("PHASE_TOKEN") == "" {
		t.Fatal("PHASE_TOKEN must be set for acceptance tests")
	}
	if os.Getenv("PHASE_TEST_APP_ID") == "" {
		t.Fatal("PHASE_TEST_APP_ID must be set for acceptance tests")
	}
}

func TestAccPhaseSecret_basic(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:  func() { testAccPreCheck(t) },
		Providers: testAccProviders,
		Steps: []resource.TestStep{
			{
				Config: testAccSecretConfig("TEST_KEY", "test-value-1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("phase_secret.test", "key", "TEST_KEY"),
					resource.TestCheckResourceAttr("phase_secret.test", "value", "test-value-1"),
					resource.TestCheckResourceAttrSet("phase_secret.test", "id"),
					resource.TestCheckResourceAttrSet("phase_secret.test", "version"),
				),
			},
			{
				// Second step re-applies with a changed value to exercise the
				// update path (resourceSecretUpdate), not just create.
				Config: testAccSecretConfig("TEST_KEY", "test-value-2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("phase_secret.test", "value", "test-value-2"),
				),
			},
		},
	})
}

func testAccSecretConfig(key, value string) string {
	return fmt.Sprintf(`
resource "phase_secret" "test" {
  app_id = %q
  env    = "development"
  key    = %q
  value  = %q
}
`, os.Getenv("PHASE_TEST_APP_ID"), key, value)
}
