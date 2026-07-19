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

## DNS Records

The following DNS records are managed by the `hcloud_dns` module. Additional records (e.g., for mail) can be added via the `extra_dns_records` variable - see `tofu/README.md` for examples.

| Record | Name             | Toggle                      | Default | Purpose           |
| ------ | ---------------- | --------------------------- | ------- | ----------------- |
| A/AAAA | `@` (apex)       | always on                   | -       | Main site and API |
| A      | `www` (optional) | `enable_www_record`         | false   | www redirect      |
| A/AAAA | `<server_name>`  | `enable_server_name_record` | false   | Server subdomain  |
| A/AAAA | `grafana`        | `enable_grafana_record`     | false   | Grafana UI        |
