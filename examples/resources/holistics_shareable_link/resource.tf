# Public link — no password, no expiry.
resource "holistics_shareable_link" "public" {
  resource_type = "Dashboard"
  resource_id   = 1234
  title         = "Public revenue dashboard"
}

# Password-protected with an expiry date — useful for investor or board access.
resource "holistics_shareable_link" "investors" {
  resource_type    = "Dashboard"
  resource_id      = 1234
  title            = "Q4 investor view"
  password_enabled = true
  password         = "share-with-care-only"
  expired_at       = "2026-12-31T23:59:59Z"
}

# Row-level filtered view — recipients only see rows where region = "EU".
# Useful for sharing a dashboard with an external partner while restricting
# the data they can see.
resource "holistics_shareable_link" "eu_partner" {
  resource_type = "Dashboard"
  resource_id   = 1234
  title         = "EU-only partner view"

  permission_rules = {
    row_based = [
      {
        field_path = { field_name = "region" }
        condition = {
          operator = "is"
          values   = ["EU"]
        }
      }
    ]
  }
}

# Filtered link — preset a dynamic filter so the link always shows a specific slice.
resource "holistics_shareable_link" "this_quarter" {
  resource_type = "Dashboard"
  resource_id   = 1234
  title         = "This quarter's view"

  dynamic_filter_presets = [
    {
      dynamic_filter_id = "2002"
      preset_condition = {
        operator = "is"
        values   = ["last_90_days"]
      }
    }
  ]
}
