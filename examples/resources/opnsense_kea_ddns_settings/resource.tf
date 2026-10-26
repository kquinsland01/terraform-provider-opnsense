# This owns existing saved settings, not DNS update delivery. Keep disabled
# until a separate operational review authorizes actual DDNS traffic.
resource "opnsense_kea_ddns_settings" "example" {
  enabled       = false
  manual_config = false
  server_ip     = "127.0.0.1"
  server_port   = 53001
}
