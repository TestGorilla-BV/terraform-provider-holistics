package provider

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDataScheduleResource_emailDest(t *testing.T) {
	withMockServer(t)

	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_data_schedule" "test" {
  source_type = "Dashboard"
  source_id   = 100

  schedule = {
    repeat = "0 8 * * *"
    paused = false
  }

  email_dest = {
    title      = "Daily report"
    recipients = ["team@example.com"]
    options = {
      include_header     = true
      attachment_formats = ["pdf"]
    }
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "source_type", "Dashboard"),
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "source_id", "100"),
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "schedule.repeat", "0 8 * * *"),
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "schedule.paused", "false"),
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "email_dest.title", "Daily report"),
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "email_dest.recipients.0", "team@example.com"),
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "email_dest.options.include_header", "true"),
				resource.TestCheckResourceAttrSet("holistics_data_schedule.test", "id"),
			),
		},
		{
			// Update: pause the schedule, add a second recipient.
			Config: providerConfig + `
resource "holistics_data_schedule" "test" {
  source_type = "Dashboard"
  source_id   = 100

  schedule = {
    repeat = "0 8 * * *"
    paused = true
  }

  email_dest = {
    title      = "Daily report"
    recipients = ["team@example.com", "leads@example.com"]
    options = {
      include_header     = true
      attachment_formats = ["pdf"]
    }
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "schedule.paused", "true"),
				resource.TestCheckResourceAttr("holistics_data_schedule.test", "email_dest.recipients.#", "2"),
			),
		},
		{
			ResourceName:      "holistics_data_schedule.test",
			ImportState:       true,
			ImportStateVerify: true,
		},
	})
}

func TestAccDataScheduleResource_slackDest(t *testing.T) {
	withMockServer(t)
	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_data_schedule" "slacker" {
  source_type = "QueryReport"
  source_id   = 200

  schedule = {
    repeat = "*/30 * * * *"
    paused = false
  }

  slack_dest = {
    title   = "Slack delivery"
    message = "Here's the report"
    slack_channels = [
      { name = "#analytics", type = "channel" }
    ]
  }
}
`,
			Check: resource.ComposeAggregateTestCheckFunc(
				resource.TestCheckResourceAttr("holistics_data_schedule.slacker", "slack_dest.slack_channels.0.name", "#analytics"),
				resource.TestCheckResourceAttr("holistics_data_schedule.slacker", "source_type", "QueryReport"),
			),
		},
	})
}

func TestAccDataScheduleResource_multipleDestsRejected(t *testing.T) {
	withMockServer(t)
	runResourceTest(t, []resource.TestStep{
		{
			Config: providerConfig + `
resource "holistics_data_schedule" "bad" {
  source_type = "Dashboard"
  source_id   = 1

  schedule = { repeat = "0 0 * * *", paused = false }

  email_dest = { recipients = ["a@b.com"] }
  google_sheet_dest = { sheet_url = "https://example.com", sheet_title = "x" }
}
`,
			ExpectError: regexp.MustCompile(`exactly one of email_dest`),
		},
	})
}
