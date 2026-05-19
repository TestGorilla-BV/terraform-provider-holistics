# Hard-coded membership — useful when user IDs are known up front.
resource "holistics_group" "analysts" {
  name     = "Analysts"
  user_ids = [101, 102, 103]
}

# Membership computed from a data source — every admin is auto-added to the
# "Workspace Admins" group. If a new admin is created elsewhere, the next
# `terraform apply` picks them up.
data "holistics_users" "admin_pool" {
  role            = "admin"
  exclude_deleted = true
}

resource "holistics_group" "workspace_admins" {
  name     = "Workspace Admins"
  user_ids = [for u in data.holistics_users.admin_pool.users : u.id]
}

# A group with no users — useful as a placeholder while membership is managed
# elsewhere (e.g. via Holistics SSO group sync).
resource "holistics_group" "sso_managed" {
  name = "SSO Sync Target"
}
