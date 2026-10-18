resource "opnsense_kea_dhcpv6_peer" "standby" {
  name = "standby.example.test"
  role = "standby"
  url  = "http://[2001:db8:31::2]:8000/"
}
