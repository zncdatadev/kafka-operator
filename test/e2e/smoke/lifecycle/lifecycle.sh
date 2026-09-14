#!/usr/bin/env bash
set -euo pipefail
: "${NAMESPACE:?Chainsaw must supply an isolated namespace}"
state=$(mktemp -d)
trap 'rm -rf "$state"' EXIT
k() { kubectl -n "$NAMESPACE" "$@"; }

wait_for() {
    local description=$1
    shift
    local deadline=$((SECONDS + 300))
    until "$@"; do
        if (( SECONDS >= deadline )); then
            echo "Timed out: $description" >&2
            return 1
        fi
        sleep 3
    done
}

replicas_are() {
    local name=$1 expected=$2
    k get sts "$name" -o json | python3 -c '
import json, sys
s = json.load(sys.stdin)
n = int(sys.argv[1])
assert s["spec"]["replicas"] == n
assert s.get("status", {}).get("readyReplicas", 0) == n
assert s.get("status", {}).get("replicas", 0) == n
assert s.get("status", {}).get("observedGeneration", 0) >= s["metadata"]["generation"]
' "$expected" 2>/dev/null
}

config_is() {
    k get cm lifecycle-broker-primary -o jsonpath='{.data.server\.properties}' |
        grep -qx "log.retention.hours=$1"
}

discovery_is_isolated() {
    k get cm lifecycle lifecycle-nodeport other other-nodeport -o json > "$state/discovery.json"
    k get listeners -o json > "$state/listeners.json"
    python3 - "$state" "$1" <<'PY'
import json, pathlib, sys
root = pathlib.Path(sys.argv[1])
cms = {v['metadata']['name']: v['data']['KAFKA'] for v in json.loads((root/'discovery.json').read_text())['items']}
listeners = {v['metadata']['name']: v for v in json.loads((root/'listeners.json').read_text())['items']}
for cluster, groups in [('lifecycle', sys.argv[2].split(',')), ('other', ['default'])]:
    expected = set()
    for group in groups:
        listener = listeners[f'{cluster}-broker-{group}-bootstrap']
        addresses = listener.get('status', {}).get('ingressAddresses', [])
        assert addresses, listener['metadata']['name']
        expected.update(f'{a["address"]}:{a["ports"]["kafka"]}' for a in addresses)
    assert set(cms[cluster].split(',')) == expected, (cluster, cms[cluster], expected)
    assert cms[cluster + '-nodeport'] == cms[cluster]
assert set(cms['lifecycle'].split(',')).isdisjoint(cms['other'].split(','))
PY
}

snapshot_identity() {
    k get sts,cm,svc,pdb,sa,pvc,listeners -l kafka.kubedoop.dev/cluster=lifecycle -o json |
        python3 -c 'import json,sys; print(json.dumps(sorted((v["kind"],v["metadata"]["name"],v["metadata"]["uid"]) for v in json.load(sys.stdin)["items"])))'
}

for name in lifecycle-broker-primary lifecycle-broker-secondary other-broker-default; do
    wait_for "$name ready" replicas_are "$name" 1
done
wait_for 'two-cluster discovery isolation' discovery_is_isolated primary,secondary
k get pdb lifecycle-broker -o json | python3 -c 'import json,sys; assert json.load(sys.stdin)["spec"]["maxUnavailable"] == 1'
k get sa kafkacluster-lifecycle kafkacluster-other -o json | python3 -c '
import json,sys
items=json.load(sys.stdin)["items"]
assert len({x["metadata"]["ownerReferences"][0]["uid"] for x in items}) == 2
'

echo 'Stop retains resources and continues applying configuration'
snapshot_identity > "$state/before-stop.json"
k get pvc data-lifecycle-broker-primary-0 -o jsonpath='{.metadata.uid}' > "$state/data-uid"
k patch kafkacluster lifecycle --type merge -p '{"spec":{"clusterOperation":{"stopped":true},"brokers":{"roleGroups":{"primary":{"configOverrides":{"server.properties":{"log.retention.hours":"43"}}}}}}}'
wait_for 'primary stopped' replicas_are lifecycle-broker-primary 0
wait_for 'secondary stopped' replicas_are lifecycle-broker-secondary 0
wait_for 'configuration applied while stopped' config_is 43
snapshot_identity > "$state/after-stop.json"
cmp "$state/before-stop.json" "$state/after-stop.json"
replicas_are other-broker-default 1

echo 'Resume reuses the data PVC'
k patch kafkacluster lifecycle --type merge -p '{"spec":{"clusterOperation":{"stopped":false}}}'
wait_for 'primary resumed' replicas_are lifecycle-broker-primary 1
wait_for 'secondary resumed' replicas_are lifecycle-broker-secondary 1
test "$(k get pvc data-lifecycle-broker-primary-0 -o jsonpath='{.metadata.uid}')" = "$(cat "$state/data-uid")"

echo 'Pause freezes desired-resource changes, including an explicit zero replica request'
k patch kafkacluster lifecycle --type merge -p '{"spec":{"clusterOperation":{"reconciliationPaused":true}}}'
# Let any already-running reconcile finish before making the edit under test.
sleep 10
k patch kafkacluster lifecycle --type merge -p '{"spec":{"brokers":{"roleGroups":{"primary":{"replicas":0,"configOverrides":{"server.properties":{"log.retention.hours":"44"}}}}}}}'
# Check the whole interval rather than accepting one immediate, stale observation.
for _ in {1..10}; do
    replicas_are lifecycle-broker-primary 1
    config_is 43
    sleep 2
done
k patch kafkacluster lifecycle --type merge -p '{"spec":{"clusterOperation":{"reconciliationPaused":false}}}'
wait_for 'explicit zero reaches StatefulSet' replicas_are lifecycle-broker-primary 0
wait_for 'configuration applied after unpause' config_is 44
replicas_are lifecycle-broker-secondary 1
k patch kafkacluster lifecycle --type merge -p '{"spec":{"brokers":{"roleGroups":{"primary":{"replicas":1}}}}}'
wait_for 'primary restored from zero' replicas_are lifecycle-broker-primary 1

echo 'Remove a role group and reclaim its resources without affecting another cluster'
k get pvc data-lifecycle-broker-secondary-0 -o jsonpath='{.metadata.uid} {.spec.volumeName}' > "$state/secondary-data"
k patch kafkacluster lifecycle --type json -p '[{"op":"remove","path":"/spec/brokers/roleGroups/secondary"}]'
# wait --for=delete actually waits; an immediate negative assertion could win a race.
k wait --for=delete sts/lifecycle-broker-secondary --timeout=300s
k wait --for=delete listeners/lifecycle-broker-secondary-bootstrap --timeout=300s
for resource in cm/lifecycle-broker-secondary svc/lifecycle-broker-secondary svc/lifecycle-broker-secondary-headless svc/lifecycle-broker-secondary-metrics; do
    k wait --for=delete "$resource" --timeout=180s
done
wait_for 'removed group excluded from discovery' discovery_is_isolated primary
replicas_are lifecycle-broker-primary 1
replicas_are other-broker-default 1
test "$(k get pvc data-lifecycle-broker-primary-0 -o jsonpath='{.metadata.uid}')" = "$(cat "$state/data-uid")"
test "$(k get pvc data-lifecycle-broker-secondary-0 -o jsonpath='{.metadata.uid} {.spec.volumeName}')" = "$(cat "$state/secondary-data")"
echo 'Lifecycle acceptance passed'
