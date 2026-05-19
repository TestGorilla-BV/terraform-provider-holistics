data "holistics_user" "alice" {
  id = 123
}

output "alice_email" {
  value = data.holistics_user.alice.email
}
