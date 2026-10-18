resource "opnsense_interface_assignment" "pxe" {
  # The assignment must already exist. Use only a non-management interface
  # whose addressing can safely be changed.
  name               = "opt3"
  enabled            = true
  ipv4_mode          = "staticv4"
  ipv4_address       = "192.0.2.1"
  ipv4_prefix_length = 24
  ipv4_gateway       = ""
  ipv6_mode          = "none"
}
