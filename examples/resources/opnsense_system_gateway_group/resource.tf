# Gateway names refer to existing individual gateways, not their UUIDs.
resource "opnsense_system_gateway_group" "example" {
  name         = "example_failover"
  tier_1       = ["EXAMPLE_PRIMARY"]
  tier_2       = ["EXAMPLE_SECONDARY"]
  trigger      = "downlosslatency"
  pool_options = ""
  description  = "Synthetic failover group"
}
