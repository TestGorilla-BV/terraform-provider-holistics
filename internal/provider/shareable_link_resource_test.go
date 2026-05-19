package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccShareableLinkResource_basic(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_shareable_link" "dashboard" {
  resource_type    = "Dashboard"
  resource_id      = 9001
  title            = "Quarterly report"
  password_enabled = false
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_shareable_link.dashboard", "resource_type", "Dashboard"),
				resource.TestCheckResourceAttr("holistics_shareable_link.dashboard", "resource_id", "9001"),
				resource.TestCheckResourceAttr("holistics_shareable_link.dashboard", "title", "Quarterly report"),
				resource.TestCheckResourceAttr("holistics_shareable_link.dashboard", "password_enabled", "false"),
				resource.TestCheckResourceAttrSet("holistics_shareable_link.dashboard", "id"),
			),
		},
		{
			// Update: enable password, add expiry, change title.
			Config: providerConfig + `
resource "holistics_shareable_link" "dashboard" {
  resource_type    = "Dashboard"
  resource_id      = 9001
  title            = "Public quarterly report"
  password_enabled = true
  password         = "hunter2"
  expired_at       = "2026-12-31T23:59:59Z"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_shareable_link.dashboard", "title", "Public quarterly report"),
				resource.TestCheckResourceAttr("holistics_shareable_link.dashboard", "password_enabled", "true"),
				resource.TestCheckResourceAttr("holistics_shareable_link.dashboard", "expired_at", "2026-12-31T23:59:59Z"),
			),
		},
		{
			ResourceName:      "holistics_shareable_link.dashboard",
			ImportState:       true,
			ImportStateVerify: true,
			// The API never echoes the password back, so the imported state
			// won't have it. The plan re-applies it from config.
			ImportStateVerifyIgnore: []string{"password"},
		},
	})
}
