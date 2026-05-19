data "holistics_data_source" "warehouse" {
  id = 5
}

output "warehouse_db_type" {
  value = data.holistics_data_source.warehouse.dbtype
}
