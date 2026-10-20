resource "opnsense_wireguard_peer" "this" {
  enabled        = false
  name           = "example_peer"
  public_key     = var.peer_public_key
  psk            = var.peer_psk
  tunnel_address = ["10.10.0.2/32", "fd00::2/128"]
  server_address = "vpn.example.com"
  server_port    = 51820
  keepalive      = 25
}
variable "peer_public_key" {
  type = string
}
variable "peer_psk" {
  type      = string
  sensitive = true
  default   = ""
}
