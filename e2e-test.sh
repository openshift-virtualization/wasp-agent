
set -e

c() { echo "# $@" ; }
n() { echo "" ; }
x() { echo "\$ $@" ; eval "$@" ; }
red() { echo -e "\e[0;31m$@\e[0m" ; }
green() { echo -e "\e[0;32m$@\e[0m" ; }
die() { red "FATAL: $@" ; exit 1 ; }
assert() { echo "(assert:) \$ $@" ; eval $@ || { echo "(assert?) FALSE" ; die "Assertion ret 0 failed: '$@'" ; } ; green "(assert?) True" ; }

c "Assumption: 'oc' is present and has access to the cluster"

c "Assume MCP worker is updated"
assert "oc get mcp worker -o json | jq '.status.conditions[] | select(.type == \"Updated\" and .status == \"True\")'"

c "Deploy"
x "bash to.sh deploy"
assert "oc get namespaces | grep wasp"

n
c "Wait for MCP to pickup new MC"
x "sleep 10s"
x "bash to.sh wait_for_worker_mcp_update_to_complete"

n
c "Wait for MCP to be updated"
x "oc wait mcp worker --for condition=Updated=True --timeout=15m"

n
c "Check the presence of swap"
assert "bash to.sh check_nodes | grep 4999"

n
c "Delete it"
x "bash to.sh destroy"
x "sleep 10"
x "bash to.sh wait_for_worker_mcp_update_to_complete"

n
c "Check the absence of swap"
assert "bash to.sh check_nodes | grep -E '0\\s+0\\s+0'"


n
c "The validation has passed! All is well."

green "PASS"
