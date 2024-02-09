#!/usr/bin/bash
#
set -e

c() { echo "# $@" ; }
n() { echo "" ; }
x() { echo "\$ $@" ; eval "$@" ; }
red() { echo -e "\e[0;31m$@\e[0m" ; }
green() { echo -e "\e[0;32m$@\e[0m" ; }
die() { red "FATAL: $@" ; exit 1 ; }
assert() { echo "(assert:) \$ $@" ; eval $@ || { echo "(assert?) FALSE" ; die "Assertion ret 0 failed: '$@'" ; } ; green "(assert?) True" ; }

c "Assumption: 'oc' is present and has access to the cluster"

if false; then
c "Ensure that all MCP workers are updated"
assert "oc get mcp worker -o json | jq -e '.status.conditions[] | select(.type == \"Updated\" and .status == \"True\")'"
c "Ensure there is no swap"
assert "bash to.sh check_nodes | grep -E '0\\s+0\\s+0'"

c "Deploy"
x "bash to.sh deploy"
assert "oc get namespaces | grep wasp"

n
c "Wait for MCP to pickup new MC"
x "bash to.sh wait_for_mcp"
fi
n
c "Check the presence of swap"
assert "grep 'Environment=SWAP_SIZE_MB=5000' manifests/machineconfig-add-swap.yaml"
assert "bash to.sh check_nodes | grep -E '4999\\s+0\\s+4999'"

n
c "Check if the container's memory.swap.max is configured properly"
c "[[ \`oc run check-has-swap-max --image=quay.io/fdeutsch/wasp-operator-prototype --rm -it --command -- cat /sys/fs/cgroup/memory.swap.max\` == 'max' ]]"

n
c "Run a workload to force swap utilization"
x "oc apply -f examples/stress.yaml"
x "oc wait deployment stress --for condition=Available=True"
c "Give it some time to generate some load"
x "sleep 60"
x "bash to.sh check_nodes"
assert "[[ \`bash to.sh check_nodes | awk '{print \$3;}' | grep -E '[0-9]+' | paste -sd+ | bc\` > 0 ]]"

n
c "Delete it"
x "bash to.sh destroy"
x "bash to.sh wait_for_mcp"

n
c "Check the absence of swap"
assert "bash to.sh check_nodes | grep -E '0\\s+0\\s+0'"


n
c "The validation has passed! All is well."

green "PASS"
