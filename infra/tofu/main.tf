module "server" {
  source = "./modules/hcloud_server"

  server_name         = var.server_name
  server_type         = var.server_type
  image               = var.image
  location            = var.location
  user_name           = var.user_name
  ssh_port            = var.ssh_port
  ssh_authorized_keys = var.ssh_authorized_keys
  ssh_public_key_file = var.ssh_public_key_file
  domain_tld          = var.domain_tld
  enable_rdns         = var.enable_rdns
  enable_firewall     = var.enable_firewall
}

module "dns" {
  source = "./modules/hcloud_dns"

  domain_tld                = var.domain_tld
  server_name               = var.server_name
  server_ipv4               = module.server.server_ipv4
  server_ipv6               = module.server.server_ipv6
  enable_www_record         = var.enable_www_record
  enable_server_name_record = var.enable_server_name_record
  enable_grafana_record     = var.enable_grafana_record
  extra_records             = var.extra_dns_records
}
