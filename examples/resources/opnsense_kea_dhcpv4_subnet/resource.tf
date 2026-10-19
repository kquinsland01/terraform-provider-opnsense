resource "opnsense_kea_dhcpv4_subnet" "lan" {
  subnet        = "192.0.2.0/24"
  pools         = "192.0.2.100-192.0.2.199"
  auto_collect  = false
  dns_servers   = ["192.0.2.53", "192.0.2.54"]
  domain_name   = "example.test"
  domain_search = ["example.test", "services.example.test"]
  routers       = ["192.0.2.1"]
  static_routes = [{ destination = "198.51.100.1", router = "192.0.2.1" }]
  ntp_servers   = ["192.0.2.20"]
  time_servers  = ["192.0.2.21"]
  next_server   = "192.0.2.22"
  tftp_server   = "tftp.example.test"
  tftp_bootfile = "bootx64.efi"
  description   = "Managed DHCPv4 subnet"
}
