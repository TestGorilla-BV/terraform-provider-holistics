package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccUserAttributeResource_basic(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_user_attribute" "test" {
  name           = "department"
  attribute_type = "string"
  label          = "Department"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_user_attribute.test", "name", "department"),
				resource.TestCheckResourceAttr("holistics_user_attribute.test", "attribute_type", "string"),
				resource.TestCheckResourceAttr("holistics_user_attribute.test", "label", "Department"),
				resource.TestCheckResourceAttrSet("holistics_user_attribute.test", "id"),
			),
		},
		{
			// Update label + add description
			Config: providerConfig + `
resource "holistics_user_attribute" "test" {
  name           = "department"
  attribute_type = "string"
  label          = "Org Department"
  description    = "Where the user works"
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_user_attribute.test", "label", "Org Department"),
				resource.TestCheckResourceAttr("holistics_user_attribute.test", "description", "Where the user works"),
			),
		},
		{
			ResourceName:      "holistics_user_attribute.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	})
}
