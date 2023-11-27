
ERESNAME=example.com/swap

_allproccgroups() {
cat /proc/*/cgroup \
	| grep "\.slice" \
	| cut -d ":" -f3 \
	| sed "s#^#/sys/fs/cgroup/#" \
	| while read FN ; do [[ -f "$FN/memory.swap.max" ]] && echo $FN ; done
}

cgroups_with_pod() {
	_allproccgroups | grep kubepods.slice | grep -v conmon
}

cgroups_without_pod() {
	_allproccgroups | grep -v kubepods.slice
}


configureNoSwap() { # PATH
	configureSwap $1 0 ; }

configureSwap() { # PATH VAL
	local PATH=$1
	local QUANTITY=$2
	[[ -z "$QUANTITY" ]] && { echo "No swap quantity" ; return ; }
	echo "$QUANTITY > $PATH/memory.swap.max" ; }

containerNameFromPath() { # PATH
	 egrep -o "crio-[^.]*" | cut -d "-" -f2 ; }

getSwapValueFromAPIForPod() {
	# $@ should be <namespace> <pod>
	local POD_NAMESPACE="$1"
	local POD_NAME="$2"
	local CONTAINER_ID="$3"
	local CONTAINER_NAME=$(kubectl get -o json pod -n $POD_NAMESPACE $POD_NAME \
		| jq -r ".status.containerStatuses[] | select( .containerID | contains(\"$CONTAINER_ID\")) | .name")
	local SWAP_QUANTITY_BYTES=$(kubectl get -o json pod -n $POD_NAMESPACE $POD_NAME \
		 | jq -r ".spec.containers[] | select(.name == \"$CONTAINER_NAME\") | .resources.requests[\"$ERESNAME\"]")
	echo ${SWAP_QUANTITY_BYTES/null/}
}

getPodNamespaceNameFromCgroupPath() {
	# $1 should be path to cgroup in /sys
	POD_UID=$(echo "$1" | grep -oE -- "-pod[^.]*" | sed -e "s/^-pod//" -e "s/_/-/g")
	CONTAINER_ID=$(echo "$1" | grep -oE -- "crio-[0-9a-f]*" | sed -e "s/crio-//")
	POD_NAMESPACE=$(cat "/run/crio/$CONTAINER_ID/config.json" | jq -r '.annotations["io.kubernetes.pod.namespace"]')
	POD_NAME=$(cat "/run/crio/$CONTAINER_ID/config.json" | jq -r '.annotations["io.kubernetes.pod.name"]')
	echo "$POD_NAMESPACE $POD_NAME $CONTAINER_ID"
}

addSwapToThisNode() {
local SWAP_BYTES=$(cat /proc/swaps | grep swap.file | awk '{print $3;}')
curl --header "Content-Type: application/json-patch+json" \
  --request PATCH \
  --data "[{\"op\": \"add\", \"path\": \"/status/capacity/${ERESNAME/\//~1}\", \"value\": \"$SWAP_BYTES\"}]" \
  http://localhost:8001/api/v1/nodes/$(hostname)/status
}

# FIXME we shoud set noswap for all cgroups, not just leaves, just to be sure
cgroups_without_pod | while read FN ; do configureNoSwap $FN ; done
#cgroups_with_pod | while read FN ; do configureSwap $FN $(getSwapValueFromAPIForPod $(getPodNamespaceNameFromCgroupPath $FN)) ; done
