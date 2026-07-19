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

resource "hcloud_rdns" "ipv4" {
  count      = var.enable_rdns ? 1 : 0
  server_id  = hcloud_server.server.id
  ip_address = hcloud_server.server.ipv4_address
  dns_ptr    = "${var.domain_tld}"
}

resource "hcloud_rdns" "ipv6" {
  count      = var.enable_rdns ? 1 : 0
  server_id  = hcloud_server.server.id
  ip_address = hcloud_server.server.ipv6_address
  dns_ptr    = "${var.domain_tld}"
}
