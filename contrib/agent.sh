#!/usr/bin/bash
#
# Expected to be set by DS
FSROOT=${FSROOT:-/host}
DEBUG=${DEBUG}
DRY=${DRY}

STRATEGY=${STRATEGY:-ortho}
SWAPPINESS=${SWAPPINESS:-60}
SWAP_SIZE_MB=${SWAP_SIZE_MB:-4000}  # RISKY dos node? was 100


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
		#fallocate -z -l ${SWAP_SIZE_MB}M $SWAPFILE
		chmod 0600 $SWAPFILE
		mkswap $SWAPFILE
		swapon $SWAPFILE
	}

	[[ -n "$SWAPPINESS" ]] && { echo "$SWAPPINESS" > "$FSROOT/proc/sys/vm/swappiness"; }

	local SWAP_KBYTES=$(grep wasp.file /proc/swaps | awk '{print $3;}')
	local SWAP_MBYTES=$(( $SWAP_KBYTES / 1024 ))

	# Announce resource
	addExtendedResource $ERESOURCE $SWAP_MBYTES
	kill $OCPID

	)
}


install_oci_hook() {
	# FIXME we shoud set noswap for all cgroups, not just leaves, just to be sure
	echo "installing hook"

	# Disable swap for system.slice
	echo 0 > $FSROOT/sys/fs/cgroup/system.slice/memory.swap.max

	cp hook.sh $FSROOT/opt/oci-hook-swap.sh
	cp hook.json $FSROOT/run/containers/oci/hooks.d/swap-for-burstable.json
}

main() {
	# FIXME hardlinks are broken if FSROOT is used, but we need it
	[[ ! -d /run/containers ]] && ln -s $FSROOT/run/containers /run/containers

	addSwapToThisNode
	install_oci_hook

	echo "Done"

	sleep inf
}

swaptop() {
	while sleep 0.3 ; do D=$(free -m ; find /sys/fs/cgroup -name memory.swap.current | while read FN ; do [[ -f "$FN" && "$(cat $FN)" -gt 0 ]] && { echo -n "$FN " ; cat $FN ; } ; done) ; clear ; echo "$D" ; done
}

${@:-main}
