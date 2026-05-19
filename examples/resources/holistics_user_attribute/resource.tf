# String attribute — common for free-form metadata like department or team.
resource "holistics_user_attribute" "department" {
  name           = "department"
  attribute_type = "string"
  label          = "Department"
  description    = "User's department"
}

# Number attribute — useful for row-level filtering (e.g. show rows only for
# customers below the user's tier).
resource "holistics_user_attribute" "customer_tier" {
  name           = "customer_tier"
  attribute_type = "number"
  label          = "Customer Tier"
}

# Boolean attribute — flag-style permissions.
resource "holistics_user_attribute" "can_see_pii" {
  name           = "can_see_pii"
  attribute_type = "boolean"
  label          = "Can See PII"
  description    = "Whether the user is cleared to view personally-identifiable fields."
}
