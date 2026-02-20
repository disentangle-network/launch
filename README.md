# launch

[![CI](https://github.com/disentangle-network/launch/actions/workflows/ci.yml/badge.svg)](https://github.com/disentangle-network/launch/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/disentangle-network/launch/branch/main/graph/badge.svg)](https://codecov.io/gh/disentangle-network/launch)
[![Go Report Card](https://goreportcard.com/badge/github.com/disentangle-network/launch)](https://goreportcard.com/report/github.com/disentangle-network/launch)
[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)

Deployment orchestrator for the [Disentangle Network](https://github.com/disentangle-network). Single CLI from infrastructure provisioning through FluxCD bootstrap across OCI, Talos, and bare-metal clusters with post-quantum Nebula mesh overlay.

## Commands

| Command | Description |
|---------|-------------|
| `launch setup` | Initialize config and verify environment |
| `launch preflight` | Check required tools and credentials |
| `launch infra {init,plan,apply,destroy,output,kubeconfig}` | OCI infrastructure via OpenTofu |
| `launch fleet init` | Initialize GitOps fleet repository |
| `launch fleet status` | Show fleet repo state and clusters |
| `launch cluster add <name>` | Register cluster in fleet |
| `launch cluster import <name> <kubeconfig>` | Import kubeconfig from any source |
| `launch secrets init` | Bootstrap SOPS age keys |
| `launch bootstrap` | FluxCD bootstrap on cluster |
| `launch mesh init` | Initialize Nebula-PQ CA |
| `launch mesh add <name>` | Add node to mesh overlay |
| `launch status` | Show pipeline state and cluster health |

Every command displays next-step hints on completion.

## Installation

```sh
brew install disentangle-network/tap/launch
```

From source:
```sh
go install github.com/disentangle-network/launch@latest
```

## Configuration

`~/.config/launch/config.yaml`:

```yaml
oci_region: us-phoenix-1
oci_compartment_id: ocid1.tenancy.oc1..xxx
cloudflare_account_id: xxx
domain: disentangle.network
github_org: disentangle-network
github_user: privsim

repos:
  k8s_oci_foundation: /path/to/k8s-oci-foundation
  deploy: /path/to/deploy
  fleet: /path/to/fleet

cluster_name: oci-dev
environment: dev

protocol_image: ghcr.io/disentangle-network/disentangle-node
protocol_version: v0.4.0
```

Secrets (Cloudflare API token) are resolved automatically from 1Password (`op://Infrastructure/disentangle/cloudflare`), falling back to `CLOUDFLARE_API_TOKEN` env var.

## Architecture

```
launch setup → preflight → infra plan → infra apply → infra kubeconfig
                                                            ↓
                         fleet init → cluster import → cluster add
                                                            ↓
                                    secrets init → bootstrap → status
                                                       ↓
                                              mesh init → mesh add
```

Supports three deployment topologies: OCI cloud (via OpenTofu), Talos bare-metal (via Omni or talosctl), and Raspberry Pi edge nodes. All kubeconfigs merge into `~/.config/launch/kubeconfig`.

## Global Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file |
| `--verbose` / `-v` | Verbose output |
| `--dry-run` | Show commands without executing |
| `--yes` / `-y` | Non-interactive mode |

## Required Tools

Run `launch preflight` to check all prerequisites: `tofu`/`terraform`, `kubectl`, `helm`, `flux`, `gh`, `sops`, `age`, `nebula-cert`.

## License

Apache-2.0
