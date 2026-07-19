# DNS Zone
resource "hcloud_zone" "zone" {
  name = var.domain_tld
  mode = "primary"
}

# Root DNS Record (A Record)
resource "hcloud_zone_rrset" "root_record" {
  zone = hcloud_zone.zone.id
  name = "@"
  type = "A"
  ttl  = 300
  records = [
    {
      value = var.server_ipv4
    }
  ]
}

# Root DNS Record (AAAA Record)
resource "hcloud_zone_rrset" "root_aaaa_record" {
  zone = hcloud_zone.zone.id
  name = "@"
  type = "AAAA"
  ttl  = 300
  records = [
    {
      value = var.server_ipv6
    }
  ]
}

# 'www' subdomain pointing to the same server
resource "hcloud_zone_rrset" "www_record" {
  count = var.enable_www_record ? 1 : 0
  zone  = hcloud_zone.zone.id
  name  = "www"
  type  = "A"
  ttl   = 300
  records = [
    {
      value = var.server_ipv4
    }
  ]
}

# IP v4 subdomain (optional)
resource "hcloud_zone_rrset" "a_record" {
  count = var.enable_server_name_record ? 1 : 0
  zone  = hcloud_zone.zone.id
  name  = var.server_name
  type  = "A"
  ttl   = 300
  records = [
    {
      value = var.server_ipv4
    }
  ]
}

# IP v6 subdomain (optional)
resource "hcloud_zone_rrset" "aaaa_record" {
  count = var.enable_server_name_record ? 1 : 0
  zone  = hcloud_zone.zone.id
  name  = var.server_name
  type  = "AAAA"
  ttl   = 300
  records = [
    {
      value = var.server_ipv6
    }
  ]
}

# Grafana subdomain A record (optional)
resource "hcloud_zone_rrset" "grafana_record" {
  count = var.enable_grafana_record ? 1 : 0
  zone  = hcloud_zone.zone.id
  name  = "grafana"
  type  = "A"
  ttl   = 300
  records = [
    {
      value = var.server_ipv4
    }
  ]
}

# Grafana subdomain AAAA record (optional)
resource "hcloud_zone_rrset" "grafana_aaaa_record" {
  count = var.enable_grafana_record ? 1 : 0
  zone  = hcloud_zone.zone.id
  name  = "grafana"
  type  = "AAAA"
  ttl   = 300
  records = [
    {
      value = var.server_ipv6
    }
  ]
}

# Extra DNS records (mail, verification, etc.)
resource "hcloud_zone_rrset" "extra" {
  for_each = { for idx, r in var.extra_records : "${r.type}_${r.name}_${idx}" => r }

  zone    = hcloud_zone.zone.id
  name    = each.value.name
  type    = each.value.type
  ttl     = each.value.ttl
  records = [for value in each.value.values : { value = value }]
}
