output "server_ipv4" {
  description = "IPv4 address of the Hetzner server"
  value       = module.server.server_ipv4
}

output "server_ipv6" {
  description = "IPv6 address of the Hetzner server"
  value       = module.server.server_ipv6
}

output "server_id" {
  description = "ID of the Hetzner server"
  value       = module.server.server_id
}

output "dns_zone_id" {
  description = "ID of the DNS zone"
  value       = module.dns.dns_zone_id
}

output "dns_nameservers" {
  description = "Nameservers for the DNS zone"
  value       = module.dns.dns_nameservers
}

output "fqdn_server" {
  description = "Fully qualified domain name of the server"
  value       = var.enable_server_name_record ? "${var.server_name}.${var.domain_tld}" : null
}

output "ssh_connection_instructions" {
  description = "SSH connection configuration for the Hetzner server"
  value       = <<EOF
When adding this to '~/.ssh/config', the SSH connection can be established with 'ssh hetzner-${var.server_name}':

Host hetzner-${var.server_name}
    HostName ${module.server.server_ipv4}
    User ${var.user_name}
    Port ${var.ssh_port}
    PreferredAuthentications publickey
    IdentityFile ${var.ssh_public_key_file}
    IdentitiesOnly yes
EOF
}
