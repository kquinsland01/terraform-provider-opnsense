resource "opnsense_unbound_domain_override" "corp" {
  domain      = "corp.example.com"
  type        = "query"
  server      = "10.0.0.53"
  server_port = 53
}

resource "opnsense_unbound_domain_override" "private_dot" {
  domain      = "private.example.com"
  type        = "dot"
  server      = "192.0.2.53"
  server_port = 853

  # Blank is valid and deliberately disables certificate-name verification.
  # Set this to the upstream certificate name whenever possible.
  verify_cn = "dns.example.com"
}
