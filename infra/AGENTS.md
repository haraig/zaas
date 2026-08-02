# Infrastructure - AI Assistant Configuration

## Overview

Infrastructure-as-code for deploying ZaaS to Hetzner Cloud using OpenTofu (Terraform-compatible).

## Structure

```
infra/
└── tofu/           # OpenTofu configuration for Hetzner Cloud
```

## Conventions

- **Tool:** OpenTofu (`tofu` CLI), not Terraform
- **Provider:** Hetzner Cloud (`hetznercloud/hcloud`)
- **State:** Local state by default. State files are excluded via `.gitignore`
- **Naming:** Use `snake_case` for resource names and variables
- **Variables:** All configurable values in `variables.tf` with descriptions and sensible defaults
- **Outputs:** Expose useful values (IP addresses, URLs) in `outputs.tf`
- **Secrets:** Never commit secrets or `.tfstate` files -- use `.gitignore`
- **Formatting:** Run `tofu fmt` before committing
- **Makefile:** Use `make infra-init/plan/apply/destroy` from the repo root as shortcuts

## Firewall

The `hcloud_server` module attaches a Hetzner Cloud Firewall (`enable_firewall`, default `true`) allowing only inbound SSH (`ssh_port`), HTTP, HTTPS and ICMP.

**This is the authoritative ingress control, not ufw.** Docker publishes container ports through the `FORWARD` chain, which never traverses the `INPUT` chain where ufw's rules live, so ufw cannot restrict any published container port. The cloud firewall is enforced outside the host and is not bypassable that way.

Two rules follow from this:

- **Exposing a port publicly needs a firewall rule here *and* a `ports:` entry in `deploy/docker-compose.yaml`.** Doing only one of the two either silently does nothing or silently exposes the service. Non-public services must bind `127.0.0.1` in compose.
- **Never add an `out` rule.** A Hetzner firewall leaves egress unrestricted only while it has zero outbound rules; adding even one drops everything else outbound, breaking ACME certificate issuance, GHCR image pulls, SMTP delivery and alert notifications.

Attachment uses a separate `hcloud_firewall_attachment` resource rather than the server's `firewall_ids` argument, so applying it never touches `hcloud_server` and cannot trigger a replacement.

## DNS Records

The following DNS records are managed by the `hcloud_dns` module. Additional records (e.g., for mail) can be added via the `extra_dns_records` variable - see `tofu/README.md` for examples.

| Record | Name             | Toggle                      | Default | Purpose           |
| ------ | ---------------- | --------------------------- | ------- | ----------------- |
| A/AAAA | `@` (apex)       | always on                   | -       | Main site and API |
| A      | `www` (optional) | `enable_www_record`         | false   | www redirect      |
| A/AAAA | `<server_name>`  | `enable_server_name_record` | false   | Server subdomain  |
| A/AAAA | `grafana`        | `enable_grafana_record`     | false   | Grafana UI        |
