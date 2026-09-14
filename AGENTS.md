<!-- Generated: 2026-04-06 | Updated: 2026-04-06 -->

# kafka-operator

## Purpose
Manages ZooKeeper-backed Apache Kafka 3.x deployments on Kubernetes. Handles broker role groups, configuration, persistent storage, listeners, TLS, logging, and lifecycle management. KRaft controller roles are not implemented.

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
- Unit tests use Ginkgo/Gomega; `make test` provisions envtest assets.
- Chainsaw E2E tests in `test/e2e/` require an isolated kind cluster.
- `make framework-upgrade-e2e` verifies pre-framework upgrade and rollback with retained messages and data PVCs; use a separate empty kind cluster.

### Common Patterns
- Controllers in `internal/controller/`
- CRDs use `v1alpha1` API version
- Follows `operator-go` `GenericReconciler` pattern
- Security/TLS logic isolated in `internal/security/`

## Dependencies

### Internal
- `../operator-go` — Shared operator framework (`github.com/zncdatadev/operator-go v0.13.0`)

### External
- `sigs.k8s.io/controller-runtime v0.23.3`
- `k8s.io/client-go v0.35.4`
- `k8s.io/api v0.35.4`
- Use the Go version in `go.mod` for local tools (Chainsaw uses Go testing internals).

### AI Worktree Development Mode

**IMPORTANT**: When making code changes, work in a worktree under `.worktree/`, NOT in the main working directory.

#### Workflow
1. Create worktree: `git worktree add .worktree/<branch-name> -b <branch-name>`
2. Work in `.worktree/<branch-name>/` directory
3. Test: `cd .worktree/<branch-name> && make lint && make test`
4. Commit changes in the worktree
5. Push and create PR from the worktree branch
6. Cleanup: `git worktree remove .worktree/<branch-name>`

#### Rules
- NEVER modify files directly in the main working directory
- Each task gets its own worktree with a descriptive branch name
- Run `make generate` if API structs are modified
- Run `make lint && make test` before committing

<!-- MANUAL: Any manually added notes below this line are preserved on regeneration -->
