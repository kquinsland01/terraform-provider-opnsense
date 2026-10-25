resource "opnsense_dnsmasq_settings" "example" {
  # Set global settings explicitly for your appliance before applying.
  enable = true

  dhcp = {
    authoritative = true
    reply_delay   = 0
    log_dhcp      = false
    log_quiet     = true
    # Omitted nested settings are preserved. An explicit empty set clears
    # DHCP interface exclusions; interface keys (not display labels) are used.
    no_interface = []
  }
}
