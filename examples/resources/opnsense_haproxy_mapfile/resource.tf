# Domain map used by a map_use_backend HAProxy action.
resource "opnsense_haproxy_mapfile" "domain_map" {
  name        = "domain-map"
  description = "Host to backend routing map"
  type        = "dom"
  content     = <<-EOT
    grafana.example.com metrics-backend
    argocd.example.com deploy-backend
    portal.example.com app-backend
    www.portal.example.com app-backend
  EOT
}
