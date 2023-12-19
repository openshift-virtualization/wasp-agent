#!/usr/bin/bash
#
# Expected to be set by DS
FSROOT=${FSROOT:-/host}
DEBUG=${DEBUG}
DRY=${DRY}

STRATEGY=${STRATEGY:-ortho}
SWAPPINESS=${SWAPPINESS:-60}
SWAP_SIZE_MB=${SWAP_SIZE_MB:-4000}  # RISKY dos node? was 100


ERESNAME=node.kubevirt.io/swap

_allproccgroups() {
cat $FSROOT/proc/*/cgroup \
	| grep "\.slice" \
	| cut -d ":" -f3 \
	| sed "s#^#$FSROOT/sys/fs/cgroup/#" \
	| while read FN ; do [[ -f "$FN/memory.swap.max" ]] && echo $FN ; done \
	| sort -u
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
	if [[ -n "${DRY}" ]];
	then
		echo "CFGSET DRY!"
	else
		echo "CFGSET"
		echo "$VAL" > "$FN"
	fi
		echo "$ echo $VAL > $FN"
}

_configureSwap() { # FN SWAP_REQUEST MEMORY_REQUESTS
	# this function expects VARs to be passed, i.e.
	# FN=/foo SWAP_REQUEST=234 MEMORY_REQUESTS=4M
	[[ -n "$DEBUG" ]] && echo "configureSwap $@"
	eval $@

	[[ -z "$SWAP_REQUEST" ]] && { [[ -n "${DEBUG}" ]] && echo "No swap quantity" ; return ; }
	local MEM_MAX SWAP_MAX
	case ${STRATEGY} in
		"force-hard")
		# A workload is force to swap beyond requests
			MEM_MAX=$MEMORY_REQUEST
			SWAP_MAX=$SWAP_REQUEST
		;;
		# the following two likely require node swapp threshold configuration
		"ortho" | "ortho-only")
		# A workload gets additional swap, which it can use under node pressure
			#MEM_MAX=max  # no need to touch actually
			SWAP_MAX=$SWAP_REQUEST
		;;
		"allow-spike")
		# A workload can spike into a node's memory, will be squeezed into swap
			MEM_HIGH=$MEMORY_REQUEST
			#MEM_MAX=max  # no need to touch actually
			SWAP_MAX=$SWAP_REQUEST
		;;
	esac

	# NOTE we add the M suffix, as swap can only be allocated on a M granularity (or page?) ... at least not on a byte basis
	SWAP_MAX="${SWAP_MAX/k/000}"  # prag fix for kube api providing 2k for 2000, 2k not recognized by kernel
	_cg_set "${SWAP_MAX}M" "$FN/memory.swap.max" ;

	# FIXME naively dropping kube quantity into cgroups memory.max, often works ...
	[[ ! -z "$MEM_MAX" ]] && { _cg_set "$MEM_MAX" "$FN/memory.max" ; }
	[[ ! -z "$MEM_HIGH" ]] && { _cg_set "$MEM_HIG" "$FN/memory.high" ; }

	# https://facebookmicrosites.github.io/cgroup2/docs/memory-controller.html
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

removeExtendedResource() {
	local RESNAME=${1/\//~1}
	curl -s --header "Content-Type: application/json-patch+json" \
	  --request PATCH \
	  --data "[{\"op\": \"remove\", \"path\": \"/status/capacity/${RESNAME}\"}]" \
	  http://localhost:8001/api/v1/nodes/$NODE_NAME/status
}

addExtendedResource() {
	local RESNAME=${1/\//~1}
	local QUANTITY=$2
	curl -s --header "Content-Type: application/json-patch+json" \
	  --request PATCH \
	  --data "[{\"op\": \"add\", \"path\": \"/status/capacity/${RESNAME}\", \"value\": \"$QUANTITY\"}]" \
	  http://localhost:8001/api/v1/nodes/$NODE_NAME/status
}

addSwapToThisNode() {
	kubectl proxy &
	local OCPID=$!
	sleep 1

	# Cleanup any potential resource until we've prepped the system
	removeExtendedResource $ERESNAME

	(set -x
	# For debug
	grep wasp.file /proc/swaps && swapoff -v $FSROOT/var/tmp/wasp.file

	grep wasp.file /proc/swaps || {
		local SWAPFILE=$FSROOT/var/tmp/wasp.file
		dd if=/dev/zero of=$SWAPFILE bs=1M count=$SWAP_SIZE_MB
		chmod 0600 $SWAPFILE
		mkswap $SWAPFILE
		swapon $SWAPFILE
	}

	[[ -n "$SWAPPINESS" ]] && { _cg_set "$SWAPPINESS" "$FSROOT/proc/sys/vm/swappiness"; }

	local SWAP_KBYTES=$(grep wasp.file /proc/swaps | awk '{print $3;}')
	local SWAP_MBYTES=$(( $SWAP_KBYTES / 1024 ))

	# Announce resource
	addExtendedResource $ERESOURCE $SWAP_MBYTES
	kill $OCPID

	)
}


set_groundtruth() {
	# FIXME we shoud set noswap for all cgroups, not just leaves, just to be sure
	echo "Setting groundtruth"
	all_no_swap() { while read FN ; do configureNoSwap $FN ; done ; }
	case "$STRATEGY" in
		"ortho-only")
			echo "This strategy does not change any host settings, only wrkld"
			;;
		*)
			cgroups_without_pod | all_no_swap
			;;
	esac

	# Disable swap for system.slice
	echo 0 > $FSROOT/sys/fs/cgroup/system.slice
}

poke_holes() {
	echo "Poking holes"
	cgroups_with_pod | while read FN ; do
		[[ -n "$DEBUG" ]] && {
			echo "Processing $FN" >&2
			echo "  $(getPodNamespaceNameFromCgroupPath $FN)"
		}
		_configureSwap FN=$FN $(getSwapValueFromAPIForPod $(getPodNamespaceNameFromCgroupPath $FN))
		done
}

main() {
	# FIXME hardlinks are broken if FSROOT is used, but we need it
	[[ ! -d /run/containers ]] && ln -s $FSROOT/run/containers /run/containers

	addSwapToThisNode

	while true;
	do
		sleep 3

		set_groundtruth
		poke_holes

		${LOOP:-false} || break;
	done
}

swaptop() {
	while sleep 0.3 ; do D=$(free -m ; find /sys/fs/cgroup -name memory.swap.current | while read FN ; do [[ -f "$FN" && "$(cat $FN)" -gt 0 ]] && { echo -n "$FN " ; cat $FN ; } ; done) ; clear ; echo "$D" ; done
}

${@:-main}
