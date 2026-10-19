resource "opnsense_kea_dhcpv6_pd_pool" "clients" {
  subnet_id     = opnsense_kea_dhcpv6_subnet.lan.id
  prefix        = "2001:db8:31::"
  prefix_len    = 48
  delegated_len = 64
  description   = "Client delegations"
}
