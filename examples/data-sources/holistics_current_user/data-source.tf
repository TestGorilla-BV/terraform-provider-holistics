data "holistics_current_user" "me" {}

output "running_as" {
  value = data.holistics_current_user.me.email
}
