# Observation only: this never saves settings or reconfigures Kea.
data "opnsense_kea_ddns_settings" "example" {}

output "ddns_server_port" {
  value = data.opnsense_kea_ddns_settings.example.server_port
}
