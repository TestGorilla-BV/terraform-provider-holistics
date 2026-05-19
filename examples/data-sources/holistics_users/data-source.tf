# Filter by role — find all active analysts.
data "holistics_users" "analysts" {
  role            = "analyst"
  exclude_deleted = true
}

# Search by name/email/initials — supports partial matches against any of those.
data "holistics_users" "alices" {
  search_term     = "alice"
  exclude_deleted = true
}

# Filter by explicit IDs — useful when reconciling state with a known list.
data "holistics_users" "leadership" {
  ids = [101, 102, 103]
}

output "analyst_emails" {
  value = [for u in data.holistics_users.analysts.users : u.email]
}
