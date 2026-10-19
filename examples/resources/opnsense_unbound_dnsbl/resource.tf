resource "opnsense_unbound_dnsbl" "blocklist" {
  enabled     = true
  safe_search = true
  type        = ["ag"]
  blocklists  = ["telemetry.example"]
  nxdomain    = true
}
