---
page_title: "Bulk user provisioning"
subcategory: ""
description: |-
  Patterns for managing many Holistics users from a single Terraform configuration.
---

# Bulk user provisioning

This guide walks through a common pattern: managing a workspace's users, groups, and per-user attributes from one Terraform configuration, driven by a single source of truth.

## Goal

Given a list of teammates (with role, department, and Slack handle), produce:

- one `holistics_user` per teammate (invited via email)
- a `holistics_group` per role (`analysts`, `admins`)
- a custom `department` attribute on each user
- a `holistics_data_schedule` that emails the daily KPI dashboard to the analysts group

## Define the team in one place

Keep the source of truth in a variable so adding or removing a teammate is a one-line change:

```terraform
variable "team" {
  description = "Holistics users to provision"
  type = map(object({
    role       = string  # admin | analyst | user
    name       = string
    department = string
  }))
  default = {
    "alice@example.com" = { role = "admin",   name = "Alice Smith", department = "Eng"   }
    "bob@example.com"   = { role = "analyst", name = "Bob Lee",     department = "Sales" }
    "carol@example.com" = { role = "analyst", name = "Carol Patel", department = "Sales" }
  }
}
```

## Invite the users

`for_each` over the map keys it by email, so each user gets a stable Terraform address (`holistics_user.team["alice@example.com"]`):

```terraform
resource "holistics_user" "team" {
  for_each = var.team

  email = each.key
  role  = each.value.role
  name  = each.value.name
}
```

## Group users by role

Filter the map down to each role and pull the resulting user IDs into a group:

```terraform
locals {
  by_role = {
    for email, attrs in var.team : attrs.role => holistics_user.team[email].id...
  }
}

resource "holistics_group" "analysts" {
  name     = "Analysts"
  user_ids = try(local.by_role["analyst"], [])
}

resource "holistics_group" "admins" {
  name     = "Admins"
  user_ids = try(local.by_role["admin"], [])
}
```

The `try(..., [])` fallback keeps things working if a role has zero members.

## Add a custom attribute

```terraform
resource "holistics_user_attribute" "department" {
  name           = "department"
  attribute_type = "string"
  label          = "Department"
}
```

Per-user attribute values aren't currently surfaced as a Terraform resource (the API exposes them as upsert-only), but the attribute *definition* is — see the `holistics_user_attribute` resource docs.

## Wire up a schedule that uses the group

```terraform
resource "holistics_data_schedule" "daily_kpi" {
  source_type = "Dashboard"
  source_id   = 1234

  schedule = {
    repeat = "0 8 * * 1-5"
    paused = false
  }

  email_dest = {
    title      = "Daily KPI report"
    recipients = [for email, _ in var.team : email if _.role == "analyst"]
    options = {
      include_header     = true
      attachment_formats = ["pdf"]
    }
  }
}
```

## Adding or removing a teammate

To onboard someone, add a line to `var.team` and run `terraform apply`. The provider invites the new user, adds them to the right group, and updates any schedule recipient lists.

To offboard, remove their entry. The provider calls `DELETE /users/{id}` (soft-delete) and the group/schedule lists shrink accordingly. If you re-add the same email later, the provider transparently restores the soft-deleted record — see the "User lifecycle" note in the [README](https://github.com/TestGorilla-BV/terraform-provider-holistics#design-notes).

## Importing existing users

If your workspace already has users you want to bring under Terraform management, you have two options:

```shell
# Import by integer ID:
terraform import 'holistics_user.team["alice@example.com"]' 123

# Or by email — the Holistics UI doesn't surface IDs prominently, so this is
# usually the faster path:
terraform import 'holistics_user.team["alice@example.com"]' alice@example.com
```

After importing, run `terraform plan` to see any drift between your config and the live workspace, and reconcile.
