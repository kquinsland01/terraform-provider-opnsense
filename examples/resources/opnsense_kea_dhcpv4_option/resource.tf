resource "opnsense_kea_dhcpv4_option" "domain" {
  code        = "15"
  encoding    = "fqdn"
  data        = "example.org"
  description = "Example domain option"
  force       = false
}
