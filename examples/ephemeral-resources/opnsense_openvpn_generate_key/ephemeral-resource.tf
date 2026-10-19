ephemeral "opnsense_openvpn_generate_key" "tls_crypt" {
  key_type = "tls-crypt"
}

# Consume .key only through an ephemeral-compatible or write-only destination.
# Ordinary resource arguments would persist the value and are rejected by Terraform.
