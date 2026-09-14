#!/usr/bin/env bash
# A destructive acceptance test confined to an empty, explicitly selected kind cluster.
set -euo pipefail
: "${KUBECONFIG:?Set KUBECONFIG to the disposable kind cluster}"
: "${CHAINSAW_CLUSTER:?Set the disposable kind cluster name}"
: "${UPGRADE_BASELINE_DIR:?Provide a checkout of the pre-framework operator}"
: "${UPGRADE_BASELINE_IMAGE:?Build the pre-framework image first}"
: "${IMG:?Build the framework image first}"
root=$(cd "$(dirname "$0")/.." && pwd)
product_version=${PRODUCT_VERSION:-3.9.0}
evidence=${UPGRADE_EVIDENCE_DIR:-$root/upgrade-evidence}
mkdir -p "$evidence"
evidence=$(cd "$evidence" && pwd)
evidence=$(mktemp -d "$evidence/run.XXXXXX")
echo "Migration evidence: $evidence"
test "$(kubectl config current-context)" = "kind-$CHAINSAW_CLUSTER"
crd=$(kubectl get crd kafkaclusters.kafka.kubedoop.dev --ignore-not-found -o name)
clusters=
if [ -n "$crd" ]; then
    clusters=$(kubectl get kafkaclusters -A -o name)
fi
if [ -n "$clusters" ]; then
    echo 'Use an empty kind cluster: existing KafkaClusters would be affected.' >&2
    exit 1
fi
namespace=kafka-framework-upgrade
if kubectl get namespace "$namespace" >/dev/null 2>&1; then
    echo "Namespace $namespace already exists; inspect the previous run before retrying." >&2
    exit 1
fi
scratch=$(mktemp -d)
k() { kubectl -n "$namespace" "$@"; }
finish() {
    local result=$?
    if (( result != 0 )); then
        k get kafkacluster,sts,pod,pvc,svc,pdb,listeners -o yaml > "$evidence/failure-resources.yaml" 2>&1 || true
        k get events --sort-by=.lastTimestamp > "$evidence/failure-events.txt" 2>&1 || true
        kubectl -n kafka-operator-system logs deployment/kafka-operator-controller-manager \
            --tail=200 > "$evidence/failure-operator.log" 2>&1 || true
    fi
    rm -rf "$scratch"
}
trap finish EXIT
wait_for() {
    local description=$1
    shift
    local deadline=$((SECONDS + 400))
    until "$@"; do
        if (( SECONDS >= deadline )); then
            echo "Timed out: $description" >&2
            k get sts,pod,pvc,svc -o wide
            k get events --sort-by=.lastTimestamp
            return 1
        fi
        sleep 5
    done
}
ready() {
    k get sts "$1" -o json | python3 -c '
import json,sys
s=json.load(sys.stdin)
assert s.get("status",{}).get("readyReplicas",0) == 1
assert s.get("status",{}).get("observedGeneration",0) >= s["metadata"]["generation"]
' 2>/dev/null
}
render_operator() {
    local source=$1 image=$2 output=$3
    local render
    render=$(mktemp -d "$scratch/render.XXXXXX")
    cp -R "$source/config" "$render/config"
    (cd "$render/config/manager" && "$root/bin/kustomize" edit set image "controller=$image")
    "$root/bin/kustomize" build "$render/config/default" > "$output"
}
deploy_operator() {
    kubectl apply -f "$1"
    kubectl -n kafka-operator-system rollout status deployment/kafka-operator-controller-manager --timeout=300s
}
snapshot() {
    k get sts,cm,svc,pdb,sa,pvc,listeners -o json > "$evidence/$1-resources.json"
    k get kafkacluster upgrade -o yaml > "$evidence/$1-cluster.yaml"
    k get pvc data-upgrade-broker-primary-0 data-upgrade-broker-secondary-0 -o json |
        python3 -c 'import json,sys; print(json.dumps(sorted((x["metadata"]["name"],x["metadata"]["uid"],x["spec"]["volumeName"]) for x in json.load(sys.stdin)["items"])))' > "$evidence/$1-data-pvcs.json"
}
read_messages() {
    # Direct partition assignment tests the data without needing a consumer-offsets
    # topic (whose Kafka default replication factor is three, above this fixture's two).
    k exec upgrade-broker-primary-0 -c kafka -- env \
        KAFKA_LOG4J_OPTS=-Dlog4j.configuration=file:/kubedoop/kafka/config/tools-log4j.properties \
        bin/kafka-console-consumer.sh \
        --bootstrap-server "$(bootstrap)" --topic migration-check --partition 0 --offset earliest \
        --consumer-property enable.auto.commit=false \
        --max-messages "$1" --timeout-ms 30000 > "$scratch/messages"
    diff -u "$evidence/expected-messages.txt" "$scratch/messages"
}
write_message() {
    printf '%s\n' "$1" | k exec -i upgrade-broker-primary-0 -c kafka -- \
        env KAFKA_LOG4J_OPTS=-Dlog4j.configuration=file:/kubedoop/kafka/config/tools-log4j.properties \
        bin/kafka-console-producer.sh --bootstrap-server "$(bootstrap)" --topic migration-check \
        --producer-property acks=all --producer-property delivery.timeout.ms=30000 --producer-property request.timeout.ms=10000
    printf '%s\n' "$1" >> "$evidence/expected-messages.txt"
}
bootstrap() {
    local address
    address=$(k get cm upgrade -o jsonpath='{.data.KAFKA}')
    test -n "$address" || return 1
    printf '%s' "$address"
}
replace_workloads() {
    kubectl -n kafka-operator-system scale deployment/kafka-operator-controller-manager --replicas=0
    kubectl -n kafka-operator-system wait --for=delete pod -l control-plane=controller-manager --timeout=180s
    for group in primary secondary; do
        local workload="upgrade-broker-$group"
        # Explicitly retain PVCs before scaling/deleting, even if a user supplied a policy.
        k patch sts "$workload" --type merge -p '{"spec":{"persistentVolumeClaimRetentionPolicy":{"whenDeleted":"Retain","whenScaled":"Retain"}}}'
        k scale sts "$workload" --replicas=0
        k wait --for=delete "pod/$workload-0" --timeout=180s
        k delete sts "$workload" --wait=true
        k delete svc "$workload" --ignore-not-found
        k delete svc "$workload-headless" --ignore-not-found
        # Delete only the legacy listener claim; never select data claims for deletion.
        k delete pvc "listener-bootstrap-$workload-0" --ignore-not-found
    done
}

render_operator "$UPGRADE_BASELINE_DIR" "$UPGRADE_BASELINE_IMAGE" "$evidence/baseline-operator.yaml"
render_operator "$root" "$IMG" "$evidence/framework-operator.yaml"
kind load docker-image --name "$CHAINSAW_CLUSTER" "$UPGRADE_BASELINE_IMAGE" "$IMG"
deploy_operator "$evidence/baseline-operator.yaml"
kubectl create namespace "$namespace"
k apply -f "$root/test/e2e/setup/zookeeper.yaml"
wait_for 'ZooKeeper ready' ready zookeepercluster-sample-server-default
cat > "$evidence/input-cluster.yaml" <<YAML
apiVersion: kafka.kubedoop.dev/v1alpha1
kind: KafkaCluster
metadata:
  name: upgrade
spec:
  image:
    productVersion: "$product_version"
  clusterConfig:
    zookeeperConfigMapName: kafka-znode
    tls:
      internalSecretClass: tls
      sslStorePassword: changeit
  brokers:
    roleconfig:
      podDisruptionBudget:
        enabled: true
        maxUnavailable: 1
    roleGroups:
      primary:
        replicas: 1
      secondary:
        replicas: 1
YAML
k apply -f "$evidence/input-cluster.yaml"
for group in primary secondary; do wait_for "baseline $group ready" ready "upgrade-broker-$group"; done
wait_for 'discovery populated' bootstrap
wait_for 'topic created with replicas on both brokers' k exec upgrade-broker-primary-0 -c kafka -- \
    bin/kafka-topics.sh --bootstrap-server "$(bootstrap)" --create --if-not-exists \
    --topic migration-check --replication-factor 2 --partitions 1
: > "$evidence/expected-messages.txt"
write_message before-upgrade
wait_for 'baseline message readable' read_messages 1
snapshot before

echo 'Upgrade with existing data PVCs and ZooKeeper metadata'
replace_workloads
k delete sa upgrade
deploy_operator "$evidence/framework-operator.yaml"
for group in primary secondary; do wait_for "framework $group ready" ready "upgrade-broker-$group"; done
wait_for 'pre-upgrade data readable' read_messages 1
write_message after-upgrade
wait_for 'framework producer and consumer' read_messages 2
snapshot after
cmp "$evidence/before-data-pvcs.json" "$evidence/after-data-pvcs.json"

echo 'Rollback by rebuilding the old resource shape, retaining the expanded CRD'
replace_workloads
k delete sa kafkacluster-upgrade
k delete pdb upgrade-broker
# Do not downgrade the CRD: its expanded status/image schema may already be persisted.
# Apply old workload RBAC/deployment only, using the saved manifest without CRDs.
python3 - "$evidence/baseline-operator.yaml" "$scratch/rollback.yaml" <<'PY'
import pathlib,sys
documents=pathlib.Path(sys.argv[1]).read_text().split('\n---\n')
pathlib.Path(sys.argv[2]).write_text('\n---\n'.join(d for d in documents if '\nkind: CustomResourceDefinition\n' not in '\n'+d))
PY
deploy_operator "$scratch/rollback.yaml"
for group in primary secondary; do wait_for "rollback $group ready" ready "upgrade-broker-$group"; done
wait_for 'messages survive rollback' read_messages 2
write_message after-rollback
wait_for 'rollback producer and consumer' read_messages 3
snapshot rollback
cmp "$evidence/before-data-pvcs.json" "$evidence/rollback-data-pvcs.json"
python3 "$root/hack/compare-upgrade-resources.py" "$evidence"
kubectl delete namespace "$namespace" --wait=true --timeout=300s
deploy_operator "$evidence/framework-operator.yaml"
printf '%s\n' "PASS: $product_version baseline -> framework -> baseline; messages and data PVC identities preserved" | tee "$evidence/result.txt"
