# Email alert — fire when revenue drops below 1000.
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
    title      = "Revenue alert"
    recipients = ["oncall@example.com"]
    options    = { body_text = "Revenue dipped below the threshold. Check the dashboard for details." }
  }
}

# Slack alert — fire when BOTH conditions hold (high error rate AND low traffic).
resource "holistics_data_alert" "anomaly_slack" {
  title       = "Anomaly: low traffic with high errors"
  source_type = "DashboardWidget"
  source_id   = "5678"

  schedule = {
    repeat = "*/30 * * * *"
    paused = false
  }

  dynamic_filter_presets = []

  viz_conditions = [
    {
      field_path  = { field_name = "error_rate" }
      aggregation = "avg"
      condition = {
        operator = "greater_than"
        values   = ["0.05"]
      }
    },
    {
      field_path  = { field_name = "request_count" }
      aggregation = "sum"
      condition = {
        operator = "less_than"
        values   = ["100"]
      }
    }
  ]

  slack_dest = {
    title   = "Traffic anomaly"
    message = "Error rate is up while traffic is down — investigate."
    slack_channels = [
      { id = "C123456" }
    ]
  }
}

# Webhook alert — page on stale data via PagerDuty Events API.
resource "holistics_data_alert" "stale_data_pagerduty" {
  title       = "Critical: data freshness SLA breach"
  source_type = "VizBlock"
  source_id   = "freshness-block-uuid"

  schedule = {
    repeat = "*/5 * * * *"
    paused = false
  }

  dynamic_filter_presets = []

  viz_conditions = [
    {
      field_path = { field_name = "minutes_since_last_update" }
      condition = {
        operator = "greater_than"
        values   = ["60"]
      }
    }
  ]

  webhook_dest = {
    endpoint = "https://events.pagerduty.com/integration/abc123/enqueue"
  }
}
