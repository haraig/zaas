output "server_ipv4" {
  description = "IPv4 address of the Hetzner server"
  value       = hcloud_server.server.ipv4_address
}

output "server_ipv6" {
  description = "IPv6 address of the Hetzner server"
  value       = hcloud_server.server.ipv6_address
}

output "server_id" {
  description = "ID of the Hetzner server"
  value       = hcloud_server.server.id
}
