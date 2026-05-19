data "holistics_tags" "all" {}

output "tag_names" {
  value = [for t in data.holistics_tags.all.tags : t.name]
}
