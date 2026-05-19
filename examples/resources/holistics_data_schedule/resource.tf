# Email delivery — daily report to a team distribution list.
resource "holistics_data_schedule" "daily_email" {
  source_type = "Dashboard"
  source_id   = 1234

  schedule = {
    repeat = "0 8 * * 1-5"
    paused = false
  }

  email_dest = {
    title      = "Daily KPI report"
    recipients = ["team@example.com"]
    options = {
      include_header     = true
      include_filters    = true
      attachment_formats = ["pdf"]
    }
  }
}

# Slack delivery — post a screenshot to a channel every Monday morning.
resource "holistics_data_schedule" "weekly_slack" {
  source_type = "Dashboard"
  source_id   = 1234

  schedule = {
    repeat = "0 9 * * 1"
    paused = false
  }

  slack_dest = {
    title = "Weekly review"
    slack_channels = [
      { id = "C123456", type = "channel" }
    ]
  }
}

# SFTP delivery — push a CSV export to a vendor every hour.
resource "holistics_data_schedule" "vendor_export" {
  source_type = "QueryReport"
  source_id   = 5678

  schedule = {
    repeat = "0 * * * *"
    paused = false
  }

  sftp_dest = {
    data_source_id = 12
    file_path      = "/exports/orders_{{date}}.csv"
    format         = "csv"
    include_header = true
    separator      = ","
  }
}

# Google Sheets delivery — sync to a shared sheet four times a day.
resource "holistics_data_schedule" "gsheet_sync" {
  source_type = "Dashboard"
  source_id   = 1234

  schedule = {
    repeat = "0 */6 * * *"
    paused = false
  }

  google_sheet_dest = {
    sheet_url   = "https://docs.google.com/spreadsheets/d/abc123/edit"
    sheet_title = "Daily KPIs"
  }
}

# Email subscription — opt-in distribution where users self-manage.
resource "holistics_data_schedule" "subscription" {
  source_type = "Dashboard"
  source_id   = 1234

  schedule = {
    repeat = "0 8 * * *"
    paused = false
  }

  email_subscription_dest = {
    recipients = ["analytics-digest@example.com"]
    options = {
      attachment_formats = ["pdf"]
    }
  }
}

# Region-filtered delivery — apply a dynamic filter preset before sending.
resource "holistics_data_schedule" "eu_team_report" {
  source_type = "Dashboard"
  source_id   = 1234

  schedule = {
    repeat = "0 8 * * *"
    paused = false
  }

  dynamic_filter_presets = [
    {
      dynamic_filter_id = "1001"
      preset_condition = {
        operator = "is"
        values   = ["EU"]
      }
    }
  ]

  email_dest = {
    recipients = ["eu-team@example.com"]
  }
}
