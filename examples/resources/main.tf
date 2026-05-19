terraform {
  required_providers {
    holistics = {
      source = "TestGorilla-BV/holistics"
    }
  }
}

data "holistics_current_user" "me" {}

data "holistics_users" "analysts" {
  role = "analyst"
}

resource "holistics_group" "analysts" {
  name     = "Analysts"
  user_ids = [for u in data.holistics_users.analysts.users : u.id]
}

resource "holistics_user_attribute" "department" {
  name           = "department"
  attribute_type = "string"
  label          = "Department"
  description    = "User's department"
}

resource "holistics_data_schedule" "daily_kpi_email" {
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
      include_header    = true
      include_filters   = true
      attachment_formats = ["pdf"]
    }
  }
}

resource "holistics_data_alert" "revenue_drop" {
  title       = "Revenue dropped below threshold"
  source_type = "DashboardWidget"
  source_id   = "5678"

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
    recipients = ["oncall@example.com"]
  }
}

resource "holistics_shareable_link" "public_dashboard" {
  resource_type    = "Dashboard"
  resource_id      = 1234
  title            = "Public revenue dashboard"
  password_enabled = true
  password         = "supersecret"
}
