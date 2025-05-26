![Baton Logo](./baton-logo.png)

# `baton-terraform-cloud` [![Go Reference](https://pkg.go.dev/badge/github.com/conductorone/baton-terraform-cloud.svg)](https://pkg.go.dev/github.com/conductorone/baton-terraform-cloud) ![main ci](https://github.com/conductorone/baton-terraform-cloud/actions/workflows/main.yaml/badge.svg)

`baton-terraform-cloud` is a connector for built using the [Baton SDK](https://github.com/conductorone/baton-sdk).

Check out [Baton](https://github.com/conductorone/baton) to learn more the project in general.

# Getting Started

## brew

```
brew install conductorone/baton/baton conductorone/baton/baton-terraform-cloud
baton-terraform-cloud
baton resources
```

## docker

```
docker run --rm -v $(pwd):/out -e BATON_DOMAIN_URL=domain_url -e BATON_API_KEY=apiKey -e BATON_USERNAME=username ghcr.io/conductorone/baton-terraform-cloud:latest -f "/out/sync.c1z"
docker run --rm -v $(pwd):/out ghcr.io/conductorone/baton:latest -f "/out/sync.c1z" resources
```

## source

```
go install github.com/conductorone/baton/cmd/baton@main
go install github.com/conductorone/baton-terraform-cloud/cmd/baton-terraform-cloud@main

baton-terraform-cloud

baton resources
```

# Data Model

`baton-terraform-cloud` will pull down information about the following resources:
- Organizations
- Users
- Teams
- Projects
- Workspaces

# Requirements
- [API Token](https://developer.hashicorp.com/terraform/cloud-docs/users-teams-organizations/api-tokens), can be any that has access to Organizations and team management
- [Standard plan](https://www.hashicorp.com/en/pricing) or higher for Terraform Cloud 



# Contributing, Support and Issues

We started Baton because we were tired of taking screenshots and manually
building spreadsheets. We welcome contributions, and ideas, no matter how
small&mdash;our goal is to make identity and permissions sprawl less painful for
everyone. If you have questions, problems, or ideas: Please open a GitHub Issue!

See [CONTRIBUTING.md](https://github.com/ConductorOne/baton/blob/main/CONTRIBUTING.md) for more details.

# `baton-terraform-cloud` Command Line Usage

```
baton-terraform-cloud

Usage:
  baton-terraform-cloud [flags]
  baton-terraform-cloud [command]

Available Commands:
  capabilities       Get connector capabilities
  completion         Generate the autocompletion script for the specified shell
  config             Get the connector config schema
  help               Help about any command

Flags:
      --address string                                   The address of the terraform instance. Default: https://app.terraform.io ($BATON_ADDRESS) (default "https://app.terraform.io")
      --client-id string                                 The client ID used to authenticate with ConductorOne ($BATON_CLIENT_ID)
      --client-secret string                             The client secret used to authenticate with ConductorOne ($BATON_CLIENT_SECRET)
      --external-resource-c1z string                     The path to the c1z file to sync external baton resources with ($BATON_EXTERNAL_RESOURCE_C1Z)
      --external-resource-entitlement-id-filter string   The entitlement that external users, groups must have access to sync external baton resources ($BATON_EXTERNAL_RESOURCE_ENTITLEMENT_ID_FILTER)
  -f, --file string                                      The path to the c1z file to sync with ($BATON_FILE) (default "sync.c1z")
  -h, --help                                             help for baton-terraform-cloud
      --log-format string                                The output format for logs: json, console ($BATON_LOG_FORMAT) (default "json")
      --log-level string                                 The log level: debug, info, warn, error ($BATON_LOG_LEVEL) (default "info")
      --otel-collector-endpoint string                   The endpoint of the OpenTelemetry collector to send observability data to (used for both tracing and logging if specific endpoints are not provided) ($BATON_OTEL_COLLECTOR_ENDPOINT)
  -p, --provisioning                                     This must be set in order for provisioning actions to be enabled ($BATON_PROVISIONING)
      --skip-full-sync                                   This must be set to skip a full sync ($BATON_SKIP_FULL_SYNC)
      --sync-resources strings                           The resource IDs to sync ($BATON_SYNC_RESOURCES)
      --ticketing                                        This must be set to enable ticketing support ($BATON_TICKETING)
      --token string                                     required: The API token used to authenticate with terraform cloud. ($BATON_TOKEN)
  -v, --version                                          version for baton-terraform-cloud

Use "baton-terraform-cloud [command] --help" for more information about a command.
```
