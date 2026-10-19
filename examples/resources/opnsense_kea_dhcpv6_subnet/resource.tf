resource "opnsense_kea_dhcpv6_subnet" "lan" {
  subnet        = "2001:db8:31::/48"
  interface     = "lan"
  dns_servers   = ["2001:db8:31::53", "2001:db8:31::54"]
  domain_search = ["example.test", "services.example.test"]
  description   = "Managed DHCPv6 subnet"
}
