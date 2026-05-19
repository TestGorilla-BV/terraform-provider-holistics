data "holistics_groups" "all" {}

# Map group name → list of user IDs, for convenient downstream lookups.
output "memberships" {
  value = { for g in data.holistics_groups.all.groups : g.name => g.user_ids }
}

# Find a specific group by name without hardcoding its ID.
output "analysts_group_id" {
  value = one([for g in data.holistics_groups.all.groups : g.id if g.name == "Analysts"])
}
