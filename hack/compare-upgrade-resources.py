"""Check the migration's public contracts and save normalized resource diffs."""

import copy
import difflib
import json
import pathlib
import sys


def resources(root, phase):
    items = json.loads((root / f"{phase}-resources.json").read_text())["items"]
    return {(item["kind"], item["metadata"]["name"]): item for item in items}


def normalize(value):
    value = copy.deepcopy(value)
    value.pop("status", None)
    metadata = value["metadata"]
    for key in ("uid", "resourceVersion", "generation", "creationTimestamp", "managedFields"):
        metadata.pop(key, None)
    metadata.get("annotations", {}).pop("kubectl.kubernetes.io/last-applied-configuration", None)
    for owner in metadata.get("ownerReferences", []):
        owner.pop("uid", None)
    if value["kind"] == "Service":
        for key in ("clusterIP", "clusterIPs", "healthCheckNodePort"):
            value["spec"].pop(key, None)
        for port in value["spec"].get("ports", []):
            port.pop("nodePort", None)
    return value


def main(root):
    before, after, rollback = (resources(root, p) for p in ("before", "after", "rollback"))
    assert ("ServiceAccount", "upgrade") not in after
    assert ("ServiceAccount", "kafkacluster-upgrade") in after
    assert ("ServiceAccount", "kafkacluster-upgrade") not in rollback
    assert ("PodDisruptionBudget", "upgrade-broker") in after
    assert ("PodDisruptionBudget", "upgrade-broker") not in rollback
    for group in ("primary", "secondary"):
        name = f"upgrade-broker-{group}"
        old, new, restored = (r["StatefulSet", name]["spec"] for r in (before, after, rollback))
        assert old["serviceName"] == name
        assert new["serviceName"] == name + "-headless"
        assert restored["serviceName"] == old["serviceName"]
        assert new["selector"]["matchLabels"] == {
            "kafka.kubedoop.dev/cluster": "upgrade",
            "kafka.kubedoop.dev/role": "broker",
            "kafka.kubedoop.dev/role-group": group,
        }
        assert restored["selector"] == old["selector"]
        for candidate in (new, restored):
            old_data = next(v for v in old["volumeClaimTemplates"] if v["metadata"]["name"] == "data")
            new_data = next(v for v in candidate["volumeClaimTemplates"] if v["metadata"]["name"] == "data")
            assert old_data["spec"] == new_data["spec"], (name, "data claim spec changed")
        assert {v["metadata"]["name"] for v in new["volumeClaimTemplates"]} == {"data"}
        assert before["Service", name]["spec"]["clusterIP"] == "None"
        assert after["Service", name]["spec"]["clusterIP"] != "None"
        assert after["Service", name + "-headless"]["spec"]["clusterIP"] == "None"
        # Stable bootstrap Listener identity keeps client addresses across the upgrade.
        for candidate in (after, rollback):
            assert candidate["Listener", name + "-bootstrap"]["metadata"]["uid"] == before["Listener", name + "-bootstrap"]["metadata"]["uid"]
    for name in ("upgrade", "upgrade-nodeport"):
        for candidate in (after, rollback):
            original = before["ConfigMap", name]["data"]
            current = candidate["ConfigMap", name]["data"]
            assert set(original) == set(current) == {"KAFKA"}
            assert original["KAFKA"] and current["KAFKA"], "discovery must not be empty"
            expected = set()
            for group in ("primary", "secondary"):
                addresses = candidate["Listener", f"upgrade-broker-{group}-bootstrap"]["status"]["ingressAddresses"]
                assert addresses
                expected.update(f'{a["address"]}:{a["ports"]["kafka"]}' for a in addresses)
            assert set(current["KAFKA"].split(",")) == expected
            # Endpoint ordering is not part of Kafka's comma-separated bootstrap contract.
            assert set(original["KAFKA"].split(",")) == set(current["KAFKA"].split(","))
    for phase, items in (("before", before), ("after", after), ("rollback", rollback)):
        normalized = [normalize(items[key]) for key in sorted(items)]
        (root / f"{phase}-normalized.json").write_text(json.dumps(normalized, indent=2, sort_keys=True) + "\n")
    for phase in ("after", "rollback"):
        diff = difflib.unified_diff(
            (root / "before-normalized.json").read_text().splitlines(True),
            (root / f"{phase}-normalized.json").read_text().splitlines(True),
            fromfile="before", tofile=phase,
        )
        (root / f"{phase}.diff").write_text("".join(diff))
    print("Migration resource contracts passed; normalized diffs saved for review")


if __name__ == "__main__":
    main(pathlib.Path(sys.argv[1]))
