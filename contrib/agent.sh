
FSROOT=${FSROOT:-/host}
ERESNAME=node.kubevirt.io/swap
DEBUG=${DEBUG}
DRY=${DRY}

_allproccgroups() {
cat $FSROOT/proc/*/cgroup \
	| grep "\.slice" \
	| cut -d ":" -f3 \
	| sed "s#^#$FSROOT/sys/fs/cgroup/#" \
	| while read FN ; do [[ -f "$FN/memory.swap.max" ]] && echo $FN ; done
}

cgroups_without_pod() {
	# FIXME conmon
	_allproccgroups | grep -v kubepods.slice
}

cgroups_with_pod() {
	_allproccgroups | grep kubepods.slice | grep -i burstable | grep -v conmon
}

configureNoSwap() { # FN
	_configureSwap FN=$1 SWAP_QUANTITY=0 ;
}

_cg_set() {
	local VAL=$1
	local FN=$2
	echo "$ echo $VAL > $FN"
	if [[ -n "${DRY}" ]];
	then
		echo "DRY!"
	else
		( set -x ; echo "$VAL" > "$FN" ; )
	fi
}

_configureSwap() { # FN SWAP_REQUEST MEMORY_REQUESTS
	# this function expects VARs to be passed, i.e.
	# FN=/foo SWAP_REQUEST=234 MEMORY_REQUESTS=4M
	eval $@

	[[ -z "$SWAP_REQUEST" ]] && { [[ -n "${DEBUG}" ]] && echo "No swap quantity" ; return ; }
	_cg_set "${SWAP_REQUEST}M" "$FN/memory.swap.max" ;
	[[ ! -z "$MEMORY_REQUEST" ]] && {
		# FIXME naively dropping kube quantity into cgroups memory.max, often works …
		_cg_set "$MEMORY_REQUEST" "$FN/memory.max" ;
	}
}

containerNameFromPath() { # PATH
	 egrep -o "crio-[^.]*" | cut -d "-" -f2 ;
}

getSwapValueFromAPIForPod() {
	# $@ should be <namespace> <pod>
	local POD_NAMESPACE="$1"
	local POD_NAME="$2"
	local CONTAINER_ID="$3"
	local POD_SPEC="$(kubectl get -o json pod -n $POD_NAMESPACE $POD_NAME)"

	local CONTAINER_NAME=$(echo "$POD_SPEC" \
		| jq -r ".status.containerStatuses[] | select( .containerID | contains(\"$CONTAINER_ID\")) | .name")

	local SWAP_REQUEST=$(echo "$POD_SPEC" \
		 | jq -r ".spec.containers[] | select(.name == \"$CONTAINER_NAME\") | .resources.requests[\"$ERESNAME\"]")

	local MEMORY_REQUEST=$(echo "$POD_SPEC" \
		 | jq -r ".spec.containers[] | select(.name == \"$CONTAINER_NAME\") | .resources.requests.memory")

	echo "SWAP_REQUEST=${SWAP_REQUEST/null/} MEMORY_REQUEST=${MEMORY_REQUEST/null/}"
}

getPodNamespaceNameFromCgroupPath() {
	# $1 should be path to cgroup in /sys
	POD_UID=$(echo "$1" | grep -oE -- "-pod[^.]*" | sed -e "s/^-pod//" -e "s/_/-/g")
	CONTAINER_ID=$(echo "$1" | grep -oE -- "crio-[0-9a-f]*" | sed -e "s/crio-//")
	POD_NAMESPACE=$(cat "$FSROOT/run/crio/$CONTAINER_ID/config.json" | jq -r '.annotations["io.kubernetes.pod.namespace"]')
	POD_NAME=$(cat "$FSROOT/run/crio/$CONTAINER_ID/config.json" | jq -r '.annotations["io.kubernetes.pod.name"]')
	echo "$POD_NAMESPACE $POD_NAME $CONTAINER_ID"
}

addSwapToThisNode() {
	(set -x
	grep wasp.file /proc/swaps || {
		local SWAPFILE=$FSROOT/var/tmp/wasp.file
		dd if=/dev/zero of=$SWAPFILE bs=1M count=100
		chmod 0600 $SWAPFILE
		mkswap $SWAPFILE
		swapon $SWAPFILE
	}

	local SWAP_KBYTES=$(grep wasp.file /proc/swaps | awk '{print $3;}')
	local SWAP_MBYTES=$(( $SWAP_KBYTES / 1024 ))

	kubectl proxy &
	local OCPID=$!
	sleep 1
	curl --header "Content-Type: application/json-patch+json" \
	  --request PATCH \
	  --data "[{\"op\": \"add\", \"path\": \"/status/capacity/${ERESNAME/\//~1}\", \"value\": \"$SWAP_MBYTES\"}]" \
	  http://localhost:8001/api/v1/nodes/$NODE_NAME/status
	kill $OCPID

	)
}

# FIXME hardlinks are broken if FSROOT is used, but we need it
[[ ! -d /run/containers ]] && ln -s $FSROOT/run/containers /run/containers

addSwapToThisNode

while true;
do

sleep 3

# FIXME we shoud set noswap for all cgroups, not just leaves, just to be sure
echo "Setting groundtruth"
cgroups_without_pod | while read FN ; do configureNoSwap $FN ; done

echo "Poking holes"
cgroups_with_pod | while read FN ; do
	echo "Processing $FN" >&2
	echo "  $(getPodNamespaceNameFromCgroupPath $FN)"
	_configureSwap FN=$FN $(getSwapValueFromAPIForPod $(getPodNamespaceNameFromCgroupPath $FN))
done

${LOOP:-false} || break;
done
