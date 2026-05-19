package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGroupsDataSource_basic(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			// Create two groups, then read them back via the data source.
			Config: providerConfig + `
resource "holistics_group" "a" {
  name     = "Analysts"
  user_ids = [10, 20]
}

resource "holistics_group" "b" {
  name = "Admins"
}

data "holistics_groups" "all" {
  depends_on = [holistics_group.a, holistics_group.b]
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("data.holistics_groups.all", "groups.#", "2"),
				resource.TestCheckTypeSetElemNestedAttrs("data.holistics_groups.all", "groups.*", map[string]string{
					"name":       "Analysts",
					"user_ids.#": "2",
				}),
				resource.TestCheckTypeSetElemNestedAttrs("data.holistics_groups.all", "groups.*", map[string]string{
					"name":       "Admins",
					"user_ids.#": "0",
				}),
			),
		},
	})
}
