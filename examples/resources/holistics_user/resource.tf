# Single user with full attribute set.
resource "holistics_user" "alice" {
  email          = "alice@example.com"
  role           = "analyst"
  name           = "Alice Smith"
  title          = "Senior Analyst"
  job_title      = "Data Lead"
  invite_message = "Welcome to the analytics team!"
  group_ids      = [holistics_group.analysts.id]
}

# Bulk-invite from a static set — use for_each so each user gets a stable
# Terraform address keyed by their email. To change the team, edit the
# toset() literal or move it into a variable.
resource "holistics_user" "team" {
  for_each = toset([
    "bob@example.com",
    "carol@example.com",
    "dave@example.com",
  ])

  email     = each.value
  role      = "user"
  group_ids = [holistics_group.analysts.id]
}

# An admin user with API key access disabled (e.g. someone who manages the
# workspace through the UI only).
resource "holistics_user" "workspace_admin" {
  email                      = "admin@example.com"
  role                       = "admin"
  name                       = "Workspace Admin"
  allow_authentication_token = false
}
