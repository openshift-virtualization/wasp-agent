#!/usr/bin/bash
#
set -e

WITH_DEPLOY=${WITH_DEPLOY:-true}
DRY=${DRY:-false}

c() { echo "# $@" ; }
n() { echo "" ; }
x() { echo "\$ $@" ; ${DRY} || eval "$@" ; }
red() { echo -e "\e[0;31m$@\e[0m" ; }
green() { echo -e "\e[0;32m$@\e[0m" ; }
die() { red "FATAL: $@" ; exit 1 ; }
assert() { echo "(assert:) \$ $@" ; { ${DRY} || eval $@ ; } || { echo "(assert?) FALSE" ; die "Assertion ret 0 failed: '$@'" ; } ; green "(assert?) True" ; }

c "Assumption: 'oc' is present and has access to the cluster"
assert "which oc"

if $WITH_DEPLOY; then
  c "Ensure that all MCP workers are updated"
  assert "oc get mcp worker -o json | jq -e '.status.conditions[] | select(.type == \"Updated\" and .status == \"True\")'"
  c "Ensure there is no swap"
  assert "bash to.sh check_nodes | grep -E '0\\s+0\\s+0'"

  n
  c "Deploy"
  x "bash to.sh deploy"
  assert "oc get namespaces | grep wasp"

  n
  c "Wait for MCP to pickup new MC"
  x "bash to.sh wait_for_mcp"
fi

n
c "Ensure the presence of swap"
assert "grep 'Environment=SWAP_SIZE_MB=5000' manifests/machineconfig-add-swap.yaml"
assert "bash to.sh check_nodes | grep -E '4999\\s+[0-9]+\\s+[0-9]+'"

# FIXME this will urn a besteffort pod
#n
#c "Check if the container's memory.swap.max is configured properly"
#assert "oc run check-has-swap-max --image=quay.io/fdeutsch/wasp-operator-prototype --rm -it --command -- cat /sys/fs/cgroup/memory.swap.max | grep -v 0"

n
c "Run a workload to force swap utilization"
x "oc apply -f examples/stress.yaml"
x "export DST_NODE=\$(oc get nodes -l node-role.kubernetes.io/worker --no-headers -o custom-columns=NAME:.metadata.name | head -n1)"
c "DST_NODE=\$DST_NODE"
x "oc get deployment stress -o json | jq \".spec.template.spec.nodeName = \\\"\$DST_NODE\\\" | .spec.replicas = 20\" | oc apply -f -"
x "oc wait deployment stress --for condition=Available=True"

n
c "Give it enough time to generate load and for the alert to start firing"
x "sleep 3m"

n
c "Ensure the alert is firing"
# https://access.redhat.com/solutions/4250221
x "export ALERT_MANAGER=\$(oc get route alertmanager-main -n openshift-monitoring -o jsonpath='{@.spec.host}')"
assert "curl -s -k -H \"Authorization: Bearer \$(oc create token prometheus-k8s -n openshift-monitoring)\"  https://\$ALERT_MANAGER/api/v1/alerts | jq -e \".data | map(select(.labels.alertname == \\\"NodeSwapping\\\")) | .[0]\""

n
c "Remove the workload"
x "oc delete -f examples/stress.yaml"

n
c "Check that some swapping took place"
assert "bash to.sh check_nodes | grep Swap | awk '{print \$3;}' | grep -E '^[^0]+'"

if $WITH_DEPLOY; then
  n
  c "Delete the operator"
  x "bash to.sh destroy"
  x "bash to.sh wait_for_mcp"

  n
  c "Check the absence of swap"
  assert "bash to.sh check_nodes | grep -E '0\\s+0\\s+0'"
fi

n
c "The validation has passed! All is well."

green "PASS"
