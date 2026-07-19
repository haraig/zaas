variable "hcloud_token" {
  type        = string
  description = "Hetzner Cloud API token"
  sensitive   = true
}

variable "ssh_public_key_file" {
  type        = string
  description = "Path to the SSH public key file for server authentication"
  default     = "~/.ssh/id_ed25519_hetzner.pub"
}

variable "user_name" {
  type        = string
  description = "Username to create on the server"
}

variable "server_name" {
  type        = string
  description = "Name to assign to the Hetzner server and used as subdomain for DNS records"

  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.server_name)) && length(var.server_name) >= 1 && length(var.server_name) <= 63
    error_message = "server_name must be 1-63 characters, lowercase letters, numbers, and hyphens only."
  }
}

variable "ssh_port" {
  type        = number
  description = "SSH port to configure on the server"
  default     = 2222
  validation {
    condition     = var.ssh_port >= 1 && var.ssh_port <= 65535
    error_message = "ssh_port must be between 1 and 65535"
  }
}

variable "ssh_authorized_keys" {
  type        = list(string)
  description = "List of authorized SSH public keys for the user. If empty, ssh_public_key_file is used."
  default     = []
}

variable "domain_tld" {
  type        = string
  description = "Top-level domain (TLD) for DNS zone and records"
  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$", var.domain_tld))
    error_message = "domain_tld must be a valid domain name"
  }
}

variable "enable_www_record" {
  type        = bool
  description = "Whether to create a www subdomain A record"
  default     = false
}

variable "enable_server_name_record" {
  type        = bool
  description = "Whether to create A/AAAA records for the server_name subdomain"
  default     = false
}

variable "enable_grafana_record" {
  type        = bool
  description = "Whether to create A/AAAA records for the grafana subdomain"
  default     = false
}

variable "enable_rdns" {
  type        = bool
  description = "Whether to create reverse DNS entries for the server IPv4 and IPv6 addresses"
  default     = false
}

variable "server_type" {
  type        = string
  description = "Hetzner server type (e.g., cx23, cx33)"
  default     = "cx23"
}

variable "image" {
  type        = string
  description = "Hetzner server image"
  default     = "ubuntu-26.04"
}

variable "location" {
  type        = string
  description = "Hetzner datacenter location"
  default     = "nbg1"
}

variable "extra_dns_records" {
  type = list(object({
    name   = string
    type   = string
    ttl    = optional(number, 300)
    values = optional(list(string), [])
  }))
  description = "Additional DNS records (e.g., MX, TXT for SPF/DKIM/DMARC). Each entry can contain multiple values for one RRset."
  default     = []
}
