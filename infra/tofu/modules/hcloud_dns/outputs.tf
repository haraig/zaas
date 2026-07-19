output "dns_zone_id" {
  description = "ID of the DNS zone"
  value       = hcloud_zone.zone.id
}

output "dns_nameservers" {
  description = "Authoritative nameservers for the DNS zone"
  value       = hcloud_zone.zone.authoritative_nameservers.assigned
}
