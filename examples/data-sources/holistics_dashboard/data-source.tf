data "holistics_dashboard" "kpi" {
  id = 1234
}

output "dashboard_url" {
  value = data.holistics_dashboard.kpi.url
}
