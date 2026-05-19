package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataAlertResource_emailDest(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_data_alert" "revenue" {
  title       = "Revenue below threshold"
  source_type = "DashboardWidget"
  source_id   = "501"

  schedule = {
    repeat = "*/15 * * * *"
    paused = false
  }

  dynamic_filter_presets = []

  viz_conditions = [
    {
      field_path = { field_name = "revenue" }
      condition = {
        operator = "less_than"
        values   = ["1000"]
      }
    }
  ]

  email_dest = {
    title      = "Revenue alert"
    recipients = ["oncall@example.com"]
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "title", "Revenue below threshold"),
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "source_type", "DashboardWidget"),
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "source_id", "501"),
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "viz_conditions.0.condition.operator", "less_than"),
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "email_dest.recipients.0", "oncall@example.com"),
				resource.TestCheckResourceAttrSet("holistics_data_alert.revenue", "id"),
			),
		},
		{
			// Update: tighten the threshold, swap recipient.
			Config: providerConfig + `
resource "holistics_data_alert" "revenue" {
  title       = "Revenue below tighter threshold"
  source_type = "DashboardWidget"
  source_id   = "501"

  schedule = {
    repeat = "*/15 * * * *"
    paused = false
  }

  dynamic_filter_presets = []

  viz_conditions = [
    {
      field_path = { field_name = "revenue" }
      condition = {
        operator = "less_than"
        values   = ["500"]
      }
    }
  ]

  email_dest = {
    recipients = ["oncall@example.com", "execs@example.com"]
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "title", "Revenue below tighter threshold"),
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "viz_conditions.0.condition.values.0", "500"),
				resource.TestCheckResourceAttr("holistics_data_alert.revenue", "email_dest.recipients.#", "2"),
			),
		},
		{
			ResourceName:      "holistics_data_alert.revenue",
			ImportState:       true,
			ImportStateVerify: true,
		},
	})
}

func TestAccDataAlertResource_webhookDest(t *testing.T) {
	withMockServer(t)
	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_data_alert" "webhook" {
  source_type = "VizBlock"
  source_id   = "abc-123"

  schedule = {
    repeat = "0 * * * *"
    paused = false
  }

  dynamic_filter_presets = []

  webhook_dest = {
    endpoint = "https://hooks.example.com/holistics"
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_data_alert.webhook", "webhook_dest.endpoint", "https://hooks.example.com/holistics"),
				resource.TestCheckResourceAttr("holistics_data_alert.webhook", "source_id", "abc-123"),
			),
		},
	})
}
