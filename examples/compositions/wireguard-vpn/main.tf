# A WireGuard server with a single road-warrior peer.

resource "opnsense_wireguard_server" "vpn" {
  name = "wg-vpn"
  # Generate into a protected file with umask 077; never log private key contents.
  # Derive the matching public key: wg pubkey < server.key > server.pub
  # Supply both via variables/secret store. The API does not derive public_key.
  private_key    = var.wireguard_server_private_key
  public_key     = var.wireguard_server_public_key
  tunnel_address = ["10.10.0.1/24"]
  port           = 51820
  peers          = [opnsense_wireguard_peer.laptop.id]
}

resource "opnsense_wireguard_peer" "laptop" {
  name           = "laptop"
  public_key     = var.laptop_public_key # peer's `wg pubkey` output
  tunnel_address = ["10.10.0.2/32"]
  keepalive      = 25

}

variable "wireguard_server_private_key" {
  type        = string
  sensitive   = true
  description = "WireGuard server private key (output of `wg genkey`)."
}

variable "laptop_public_key" {
  type        = string
  description = "Public key of the laptop peer (output of `wg pubkey`)."
}

variable "wireguard_server_public_key" {
  type        = string
  description = "Matching server public key derived securely with wg pubkey from the supplied private key."
}

output "wireguard_server_public_key" {
  value       = opnsense_wireguard_server.vpn.public_key
  description = "Public key clients use for the server peer; no private key is exposed."
}
