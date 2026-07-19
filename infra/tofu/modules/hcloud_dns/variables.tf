variable "domain_tld" {
  type        = string
  description = "Top-level domain (TLD) for DNS zone and records"
  validation {
    condition     = can(regex("^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$", var.domain_tld))
    error_message = "domain_tld must be a valid domain name"
  }
}

variable "server_name" {
  type        = string
  description = "Server name used as subdomain"
}

variable "server_ipv4" {
  type        = string
  description = "IPv4 address of the server"
}

variable "server_ipv6" {
  type        = string
  description = "IPv6 address of the server"
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

variable "extra_records" {
  type = list(object({
    name   = string
    type   = string
    ttl    = optional(number, 300)
    values = optional(list(string), [])
  }))
  description = "Additional DNS records to create (e.g., MX, TXT for SPF/DKIM/DMARC, CNAME for mail autoconfig). Each entry can contain multiple values for one RRset."
  default     = []
}
