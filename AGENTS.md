<!-- Generated: 2026-04-06 | Updated: 2026-04-06 -->

# kafka-operator

## Purpose
Manages Apache Kafka deployments on Kubernetes. Handles creation, configuration, and lifecycle management of Kafka clusters including broker and controller (KRaft) components.

## Key Files
| File | Description |
|------|-------------|
| `go.mod` | Go module dependencies (`github.com/zncdatadev/kafka-operator`) |
| `Makefile` | Build and development commands |
| `PROJECT` | Kubebuilder project metadata |
| `Dockerfile` | Operator container image build |

## Subdirectories
| Directory | Purpose |
|-----------|---------|
| `api/v1alpha1/` | CRD types: `KafkaCluster` |
| `cmd/` | Operator entry point (`main.go`) |
| `config/` | Kubernetes manifests and kustomize configs |
| `deploy/` | Helm chart for operator deployment |
| `internal/controller/` | Reconciliation controllers |
| `internal/pkg/` | Shared internal packages |
| `internal/security/` | TLS and authentication helpers |
| `internal/util/` | Utility functions |
| `test/e2e/` | End-to-end test suites |

## For AI Agents

### Working In This Directory
- Standard Kubebuilder operator structure
- Uses `github.com/zncdatadev/operator-go` framework for reconciliation
- Run `make test` for unit tests
- Run `make deploy` to deploy to cluster
- Go module: `github.com/zncdatadev/kafka-operator`

### Testing Requirements
- E2E tests in `test/e2e/`
- Requires a running Kubernetes cluster
- Uses Ginkgo/Gomega test framework

### Common Patterns
- Controllers in `internal/controller/`
- CRDs use `v1alpha1` API version
- Follows `operator-go` `GenericReconciler` pattern
- Security/TLS logic isolated in `internal/security/`

## Dependencies

### Internal
- `../operator-go` — Shared operator framework (`github.com/zncdatadev/operator-go v0.12.6`)

### External
- `sigs.k8s.io/controller-runtime v0.23.1`
- `k8s.io/client-go v0.35.0`
- `k8s.io/api v0.35.0`
- Go 1.25+

<!-- MANUAL: Any manually added notes below this line are preserved on regeneration -->
