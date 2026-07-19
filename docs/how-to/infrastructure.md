# How to Provision Infrastructure

This guide covers provisioning the Hetzner Cloud server and DNS zone using OpenTofu.

> After provisioning, follow [deploy.md](deploy.md) to bootstrap the application on the server.

## Prerequisites

- [OpenTofu](https://opentofu.org) installed (`tofu` CLI)
- A Hetzner Cloud API token (create in the Hetzner Console)
- An SSH key pair

Generate an SSH key pair if needed:

```bash
ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519_hetzner -C "hetzner" -N ""
```

## DNS Delegation

If your domain is registered elsewhere, delegate it to Hetzner's nameservers at your registrar:

| Nameserver |
| ---------- |
| `hydrogen.ns.hetzner.com` |
| `oxygen.ns.hetzner.com` |
| `helium.ns.hetzner.de` |

Propagation can take 24-48 hours. Verify with:

```bash
dig NS zaas.at
```

## Provision

```bash
cp infra/tofu/terraform.tfvars.example infra/tofu/terraform.tfvars
# Edit terraform.tfvars: set user_name, domain_tld, server_name

export TF_VAR_hcloud_token="your_token"   # prefix with a space to keep out of shell history
make infra-init
make infra-plan   # review before applying
make infra-apply
```

## Configuration Reference

Edit `infra/tofu/terraform.tfvars`:

```hcl
user_name          = "my-user"
domain_tld         = "example.com"
server_name        = "my-server"
# ssh_port = 2222
# ssh_authorized_keys = ["ssh-ed25519 AAAA... user@example.com"]
# ssh_public_key_file = "~/.ssh/id_ed25519_hetzner.pub"
# enable_www_record = false
# enable_server_name_record = false  # set true to create my-server.example.com
# enable_grafana_record = false      # set true to create grafana.example.com
```

Do not store `hcloud_token` in `terraform.tfvars`. Use `TF_VAR_hcloud_token` in a shell session.

## What Gets Provisioned

- A Hetzner server via `hcloud_server`
- A Hetzner DNS zone for `domain_tld`
- A root A/AAAA record for the apex domain
- Optional `www` A record (`enable_www_record`)
- Optional `<server_name>` A/AAAA record (`enable_server_name_record`)
- Optional `grafana` A/AAAA record (`enable_grafana_record`)

## Adding Extra DNS Records

The `extra_dns_records` variable accepts arbitrary DNS records. Useful for mail provider setup (MX, SPF, DKIM, DMARC). See `docs/reference/runbook.md` section 5 for the full Migadu DNS record set.

Example:

```hcl
extra_dns_records = [
  {
    name  = ""
    type  = "MX"
    value = "10 mail.migadu.com."
  },
  {
    name  = ""
    type  = "TXT"
    value = "\"v=spf1 include:spf.migadu.com -all\""
  },
]
```

## Destroy

```bash
export TF_VAR_hcloud_token="your_token"
make infra-destroy
```

## References

- [OpenTofu docs](https://opentofu.org/docs/)
- [Hetzner Cloud Provider](https://registry.terraform.io/providers/hetznercloud/hcloud/latest/docs)
- [Hetzner DNS documentation](https://docs.hetzner.com/networking/dns/)
