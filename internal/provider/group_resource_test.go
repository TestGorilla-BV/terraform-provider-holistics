package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGroupResource_basic(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_group" "test" {
  name     = "Analysts"
  user_ids = [10, 20]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_group.test", "name", "Analysts"),
				resource.TestCheckResourceAttrSet("holistics_group.test", "id"),
				resource.TestCheckResourceAttr("holistics_group.test", "user_ids.#", "2"),
			),
		},
		{
			// Update: rename + change user set
			Config: providerConfig + `
resource "holistics_group" "test" {
  name     = "Senior Analysts"
  user_ids = [10, 30]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_group.test", "name", "Senior Analysts"),
				resource.TestCheckResourceAttr("holistics_group.test", "user_ids.#", "2"),
				resource.TestCheckTypeSetElemAttr("holistics_group.test", "user_ids.*", "10"),
				resource.TestCheckTypeSetElemAttr("holistics_group.test", "user_ids.*", "30"),
			),
		},
		{
			// Import by integer ID.
			ResourceName:      "holistics_group.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
		{
			// Import by group name — looked up via the /groups list.
			ResourceName:      "holistics_group.test",
			ImportState:       true,
			ImportStateVerify: true,
			ImportStateId:     "Senior Analysts",
		},
	})
}

func TestAccGroupResource_noUsers(t *testing.T) {
	withMockServer(t)
	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_group" "test" { name = "Empty" }
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_group.test", "name", "Empty"),
				resource.TestCheckResourceAttr("holistics_group.test", "user_ids.#", "0"),
			),
		},
	})
}
