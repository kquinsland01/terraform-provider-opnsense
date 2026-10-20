# Import first: terraform import opnsense_wireguard_settings.this wireguard_settings
resource "opnsense_wireguard_settings" "this" {
  enabled = false
}
