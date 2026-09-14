# Upgrading to the operator-go v0.13.0 framework

This migration requires a maintenance window. Replacing the operator image alone
does not migrate an existing StatefulSet: Kubernetes rejects changes to its
immutable fields. Stop client traffic and the old operator, preserve the data
PVCs, then recreate the broker workloads with the new operator.

Kafka remains ZooKeeper-backed. Keep the Kafka product image, ZooKeeper metadata,
KafkaCluster name, namespace, and role-group names unchanged during this migration.
Do not combine it with a Kafka version upgrade or a role-group removal.

## Intentional resource and API changes

| Contract | Before | Framework |
| ------- | ------ | --------- |
| StatefulSet name | `<cluster>-broker-<group>` | Unchanged |
| StatefulSet selector | Descriptive `app.kubernetes.io/*` labels | `kafka.kubedoop.dev/{cluster,role,role-group}` |
| StatefulSet serviceName | `<cluster>-broker-<group>` | `<cluster>-broker-<group>-headless` |
| volumeClaimTemplates | `listener-bootstrap` and `data` | `data` only; listener claims use ephemeral CSI volumes |
| Same-name role-group Service | Headless | ClusterIP; separate `-headless` Service for broker DNS |
| Workload ServiceAccount | `<cluster>` | `kafkacluster-<cluster>` |
| PDB | None in the fixed baseline | One role-level `<cluster>-broker` PDB when configured |
| Bootstrap Listener | `<cluster>-broker-<group>-bootstrap` | Name retained, identity labels added |
| Discovery ConfigMaps | `<cluster>` and `<cluster>-nodeport`, key `KAFKA` | Names and key retained; endpoints filtered by cluster and current groups |
| Pod construction | Product-owned | Framework security defaults, CSI mounts, native Vector sidecar and logging wiring |
| Default logging without Vector | Console and file appenders | Console only; unused file-log volumes removed |
| Internal TLS certificate lifetime | No explicit CSI lifetime annotation | Explicit `autoTlsCertLifetime=24h0m0s` in the default fixture |
| KafkaCluster status | Legacy status | GenericClusterStatus conditions and role-group status |
| Configuration defaults | CRD defaults could mask inherited role values | Defaults applied by the operator below role and group overrides |
| Image specification | Product-local type | Shared ImageSpec; default image tracks the operator build version |

The external `spec.brokers` shape remains, but the generated CRD is not identical.
Review changed defaults and validations, expanded image/status schemas, mount
paths, security contexts, and selector-dependent policies. Existing explicitly
stored values are not removed by deleting a schema default. Pin storage capacity,
resource limits, logging settings and the full product image when reproducing an
existing deployment's effective configuration.

## Before the maintenance window

1. Save the KafkaCluster, generated resources, old operator image, deployment,
   RBAC and CRD. Record data PVC UIDs and PV bindings. Take recoverable backups of
   topic data and ZooKeeper metadata; the procedure does not replace backups.
2. Record bootstrap addresses and verify a producer/consumer round trip with
   identifiable messages. Ensure clients can reconnect after the outage.
3. Review the exact rendered diff for the cluster, including custom PodOverrides
   and security contexts. Check that retained volume ownership remains accessible
   under the new pod UID/GID and fsGroup.
4. Test the procedure on a representative copy first. The automated acceptance
   case below covers two groups, internal TLS, message retention and rollback;
   it does not cover every custom storage driver, logging setup or override.

## Upgrade procedure

Use an explicit kubeconfig and namespace in every command. Substitute the actual
operator deployment and namespace if it was installed through Helm.

```bash
export KUBECONFIG=/path/to/cluster.kubeconfig
NS=my-kafka-namespace
CLUSTER=my-kafka
OPERATOR_NS=kafka-operator-system
OPERATOR_DEPLOYMENT=kafka-operator-controller-manager

# Stop all old operator replicas before manually changing managed workloads.
kubectl -n "$OPERATOR_NS" scale deployment "$OPERATOR_DEPLOYMENT" --replicas=0
kubectl -n "$OPERATOR_NS" rollout status deployment "$OPERATOR_DEPLOYMENT" --timeout=180s
```

For each role group, first set PVC retention to Retain, drain its pods, then delete
the StatefulSet and the old headless Service. Repeat for every group before
starting the new operator. This is a cluster outage, not a rolling upgrade.

```bash
GROUP=default
STS="$CLUSTER-broker-$GROUP"
REPLICAS=$(kubectl -n "$NS" get sts "$STS" -o jsonpath='{.spec.replicas}')
kubectl -n "$NS" patch sts "$STS" --type merge \
  -p '{"spec":{"persistentVolumeClaimRetentionPolicy":{"whenDeleted":"Retain","whenScaled":"Retain"}}}'
kubectl -n "$NS" scale sts "$STS" --replicas=0
# Run in Bash and wait for every ordinal, including with Parallel pod management.
for ((ordinal=0; ordinal<REPLICAS; ordinal++)); do
  kubectl -n "$NS" wait --for=delete "pod/$STS-$ordinal" --timeout=180s
done
kubectl -n "$NS" delete sts "$STS" --wait=true
kubectl -n "$NS" delete svc "$STS"
```

Never delete `data-*` PVCs, the KafkaCluster, ZooKeeper resources or the bootstrap
Listener. Delete obsolete `listener-bootstrap-<statefulset>-<ordinal>` PVCs only
by their exact names after the old pods terminate. The former documentation's
`metadata.name notin ()` field selector is not a valid cleanup procedure.

Install the new CRD, RBAC and operator. The new StatefulSets retain their names and
reattach the existing data PVCs. Wait for every broker, verify the data PVC UIDs
and PV bindings against the saved record, inspect the discovery ConfigMaps, and
read the pre-upgrade messages before restoring client traffic. Then verify new
writes and reads. Remove the old ServiceAccount only after no pod references it.

## Rollback

Rollback also requires stopping traffic and draining the workloads. Stop the new
operator, retain PVCs, scale each StatefulSet to zero, wait for all pods to terminate,
then delete its StatefulSet, same-name ClusterIP Service and new `-headless` Service.
Redeploy the saved old operator with its RBAC and original Kafka product image.
It recreates the legacy workload shape around the retained data PVCs.
After draining the new pods, remove the now-unused `kafkacluster-<cluster>`
ServiceAccount and `<cluster>-broker` PDB (if configured); otherwise the old operator leaves
these framework resources behind.

Keep the expanded CRD while rolling back the controller: replacing it with an older
schema can prune stored fields. Restore the old CR only after reviewing differences;
do not reapply server-assigned metadata or status from a backup. Verify pre-upgrade
and post-upgrade messages, PVC identity and discovery endpoints before resuming
traffic. If a Kafka product version or data format changed at the same time, this
controller-only rollback is not sufficient.

## Executable acceptance and evidence

Use the Go version from `go.mod`, Docker, kind, kubectl, Helm and Python 3. The target
creates a dedicated cluster, builds both operator images, and refuses to reuse an
existing kind cluster. The script also rejects an unexpected kubeconfig or existing
KafkaClusters. It fixes the old source at main commit
`d18d27f4578ad4f2acf25c3e4af36f5661ccf183` (operator-go v0.12.6).

```bash
GOTOOLCHAIN=go1.25.8 make framework-upgrade-e2e \
  CHAINSAW_CLUSTER=kafka-framework-upgrade \
  CHAINSAW_KUBECONFIG=.kubeconfig-upgrade \
  KIND_K8S_VERSION=1.35.0 PRODUCT_VERSION=3.9.0
```

The test writes messages on the old operator, upgrades, reads those messages and
writes another, rolls back, and verifies both old and new messages again. It checks
data PVC UIDs/PV bindings, claim specifications, bootstrap Listener identity,
discovery endpoint sets and the expected immutable-field changes. It saves raw
resources, normalized before/after/rollback diffs and `result.txt` under
`upgrade-evidence/`; CI uploads the directory even on failure. Review remaining
normalized differences against the intentional changes above. A failed local run
keeps its namespace and cluster available for diagnosis.

The normal `broker-lifecycle` Chainsaw case additionally exercises stopped/resumed
clusters, configuration changes while stopped, reconciliation pause, explicit zero
replicas, two independent clusters and role-group cleanup.

The upgrade fixture uses Kafka 3.9.0, two single-broker groups, internal TLS and a
single-partition topic with replication factor two on default storage. Its consumer
reads the partition directly; it does not verify consumer-group offsets, long-term
certificate renewal, external TLS, Vector or custom storage/overrides across an
upgrade. Validate those separately when an existing deployment depends on them.

## Separate product limitations

AuthenticationClass resolution and a complete Kerberos SASL client authentication
flow remain unimplemented. The Kerberos deployment test does not prove authenticated
client access. KRaft is outside this migration. Track these as separate capabilities
rather than describing them as completed framework migration features.
