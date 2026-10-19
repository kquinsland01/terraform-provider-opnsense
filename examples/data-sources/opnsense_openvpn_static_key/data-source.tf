data "opnsense_openvpn_static_key" "existing" {
  id = "00000000-0000-0000-0000-000000000000"
}

# The key is marked sensitive, but like every data-source result it remains in state.
output "openvpn_static_key_mode" {
  value = data.opnsense_openvpn_static_key.existing.mode
}
