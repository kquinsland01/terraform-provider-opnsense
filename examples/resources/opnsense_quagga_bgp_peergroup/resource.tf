# Flat owner; FRR 2.0 renders policy under each activated address family.
# Keep default origination disabled unless it is separately reviewed.
resource "opnsense_quagga_bgp_peergroup" "speakers" {
  name              = "speakers"
  remote_as         = 65001
  family            = ["ipv4", "ipv6"]
  next_hop_self     = true
  default_originate = false
}
