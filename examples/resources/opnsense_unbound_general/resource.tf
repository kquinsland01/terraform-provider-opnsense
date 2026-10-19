resource "opnsense_unbound_general" "resolver" {
  enabled            = true
  port               = "53"
  hide_identity      = true
  hide_version       = true
  enable_prefetch    = true
  aggressive_nsec    = true
  acl_default_action = "allow"
  forwarding_enabled = true
}
