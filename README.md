# launch

Deployment orchestrator for the Disentangle Network. Wraps the full LOOM deployment pipeline into a single CLI that validates preconditions, executes each stage, and tracks state for resumption on failure.

## Pipeline Stages

| Stage | Command | Description |
|-------|---------|-------------|
| Preflight | `launch preflight` | Check tools and credentials |
| 1. Discover | `launch discover` | OCI resource discovery via oci-tf-bootstrap |
| 2. Infra | `launch infra [plan\|apply]` | Provision OKE cluster via k8s-oci-foundation |
| 3. Secrets | `launch secrets` | Bootstrap secrets via genesis-operator |
| 4. Deploy | `launch deploy` | Deploy nodes via Helm/FluxCD |
| All | `launch all` | Run stages 1-4 sequentially |
| Status | `launch status` | Show pipeline state and cluster health |
| Teardown | `launch teardown` | Destroy infrastructure (with confirmation) |

## Installation

### Homebrew

```sh
brew install LarsenClose/tap/launch
```

### From Source

```sh
go install github.com/disentangle-network/launch@latest
```

## Configuration

Create `~/.config/launch/config.yaml` or `.launch.yaml` in your project root:

```yaml
oci_region: us-phoenix-1
oci_compartment_id: ocid1.tenancy.oc1..xxx

cloudflare_account_id: xxx
domain: disentangle.network

github_org: disentangle-network
github_user: privsim

repos:
  oci_tf_bootstrap: /path/to/oci-tf-bootstrap
  k8s_oci_foundation: /path/to/k8s-oci-foundation
  genesis_operator: /path/to/genesis-operator
  deploy: /path/to/deploy

cluster_name: oci-prod
environment: dev

protocol_image: ghcr.io/disentangle-network/disentangle-node
protocol_version: v0.3.1
genesis_image: ghcr.io/larsenclose/genesis-operator
genesis_version: v0.1.0
```

## Global Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--config` | | Path to config file |
| `--verbose` | `-v` | Verbose output |
| `--dry-run` | | Show commands without executing |
| `--yes` | `-y` | Non-interactive mode |

## State Management

Pipeline state is tracked at `~/.config/launch/state.json`. When a stage fails, `launch all` resumes from the last incomplete stage. Use `launch all --fresh` to reset state and start over.

## Required Tools

- `oci` - OCI CLI
- `terraform` - Infrastructure provisioning
- `kubectl` - Kubernetes management
- `helm` - Helm chart deployment
- `flux` - FluxCD CLI
- `gh` - GitHub CLI
- `task` - Task runner

Run `launch preflight` to check all prerequisites.

## License

Apache-2.0
