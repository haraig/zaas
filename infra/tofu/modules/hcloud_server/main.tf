resource "hcloud_ssh_key" "default" {
  name       = "hetzner_ssh_key_${var.server_name}"
  public_key = file(var.ssh_public_key_file)
}

locals {
  resolved_ssh_authorized_keys = length(var.ssh_authorized_keys) > 0 ? var.ssh_authorized_keys : [trimspace(file(var.ssh_public_key_file))]
}

resource "hcloud_server" "server" {
  name        = var.server_name
  server_type = var.server_type
  image       = var.image
  location    = var.location
  ssh_keys    = [hcloud_ssh_key.default.id]
  user_data = templatefile("${path.module}/user_data.yaml.tftpl", {
    user_name           = var.user_name
    ssh_port            = var.ssh_port
    ssh_authorized_keys = local.resolved_ssh_authorized_keys
  })
}

# Ingress control enforced at Hetzner's edge, before packets reach the host.
#
# This is the authoritative firewall, not ufw. Docker publishes container ports
# by DNAT'ing them in nat/PREROUTING and accepting them in FORWARD via the
# DOCKER chain, which never traverses the INPUT chain where ufw's rules live -
# so ufw silently does not filter any published container port. A cloud firewall
# sits outside the host entirely and cannot be bypassed that way.
#
# Deliberately inbound-only: a Hetzner firewall leaves egress unrestricted only
# while it has zero outbound rules. Adding even one "out" rule would drop
# everything else outbound and break ACME certificate issuance, GHCR image
# pulls, SMTP delivery and alert notifications.
resource "hcloud_firewall" "server" {
  count = var.enable_firewall ? 1 : 0
  name  = "${var.server_name}-firewall"

  # SSH. Uses var.ssh_port, the same value cloud-init writes as "Port" into
  # sshd_config.d/ssh-hardening.conf, so the two cannot drift. Nothing listens
  # on port 22 - do not add a rule for it.
  rule {
    direction   = "in"
    protocol    = "tcp"
    port        = var.ssh_port
    source_ips  = ["0.0.0.0/0", "::/0"]
    description = "SSH"
  }

  rule {
    direction   = "in"
    protocol    = "tcp"
    port        = "80"
    source_ips  = ["0.0.0.0/0", "::/0"]
    description = "HTTP (Caddy, ACME http-01 challenge)"
  }

  rule {
    direction   = "in"
    protocol    = "tcp"
    port        = "443"
    source_ips  = ["0.0.0.0/0", "::/0"]
    description = "HTTPS (Caddy)"
  }

  # Ping and, more importantly, path MTU discovery.
  rule {
    direction   = "in"
    protocol    = "icmp"
    source_ips  = ["0.0.0.0/0", "::/0"]
    description = "ICMP"
  }
}

# Attaching via a separate resource rather than the server's own firewall_ids
# argument keeps the hcloud_server resource untouched, so applying this cannot
# trigger a server replacement.
resource "hcloud_firewall_attachment" "server" {
  count       = var.enable_firewall ? 1 : 0
  firewall_id = hcloud_firewall.server[0].id
  server_ids  = [hcloud_server.server.id]
}

resource "hcloud_rdns" "ipv4" {
  count      = var.enable_rdns ? 1 : 0
  server_id  = hcloud_server.server.id
  ip_address = hcloud_server.server.ipv4_address
  dns_ptr    = var.domain_tld
}

resource "hcloud_rdns" "ipv6" {
  count      = var.enable_rdns ? 1 : 0
  server_id  = hcloud_server.server.id
  ip_address = hcloud_server.server.ipv6_address
  dns_ptr    = var.domain_tld
}
