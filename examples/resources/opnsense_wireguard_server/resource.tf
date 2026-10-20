resource "opnsense_wireguard_server" "this" {
  enabled = false
  name    = "example_server"
  # Derive securely: wg pubkey < protected-private-key-file > server.pub
  # Supply the matching pair via a protected variable/secret store; never log it.
  private_key    = var.server_private_key
  public_key     = var.server_public_key
  port           = 51820
  mtu            = 1420
  dns            = ["10.10.0.1", "fd00::1"]
  tunnel_address = ["10.10.0.1/24", "fd00::1/64"]
  peers          = []
  disable_routes = false
}
variable "server_private_key" {
  type      = string
  sensitive = true
}
variable "server_public_key" {
  type        = string
  description = "Matching public key derived with wg pubkey; distribute this key to clients."
}
