#!/usr/bin/bash
#
# Expected to be set by DS
FSROOT=${FSROOT:-/host}
DEBUG=${DEBUG}
DRY=${DRY}

SWAPPINESS=${SWAPPINESS}
SWAP_SIZE_MB=${SWAP_SIZE_MB:-4000}  # RISKY dos node? was 100


_set() { echo "Setting $1 > $2" ; echo "$1" > "$2" ; }

addSwapToThisNode() {
  set -x
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

  [[ -n "$SWAPPINESS" ]] && { _set "$SWAPPINESS" "$FSROOT/proc/sys/vm/swappiness"; }
}

tune_system_slice() {
  echo "Tuning system.slice"

  # Disable swap for system.slice
  _set 0 $FSROOT/sys/fs/cgroup/system.slice/memory.swap.max

  # Set latency target to protect the root slice from io trash
  MAJMIN=$(findmnt $FSROOT/ --output MAJ:MIN -n | sed "s/:.*/:0/")  # fixme can be manually provided
  echo "Using MAJMIN $MAJMIN"
  _set "$MAJMIN target=50" $FSROOT/sys/fs/cgroup/system.slice/io.latency
}

install_oci_hook() {
  # FIXME we shoud set noswap for all cgroups, not just leaves, just to be sure
  echo "installing hook"

  cp hook.sh $FSROOT/opt/oci-hook-swap.sh
  cp hook.json $FSROOT/run/containers/oci/hooks.d/swap-for-burstable.json
}

main() {
  # FIXME hardlinks are broken if FSROOT is used, but we need it
  [[ ! -d /run/containers ]] && ln -s $FSROOT/run/containers /run/containers

  addSwapToThisNode
  tune_system_slice
  install_oci_hook

  echo "Done"

  sleep inf
}

swaptop() {
  while sleep 0.5 ; do D=$(uptime ; free -m ; find /sys/fs/cgroup -name memory.swap.current | while read FN ; do [[ -f "$FN" && "$(cat $FN)" -gt 0 ]] && { echo -n "$FN " ; numfmt --to=iec-i --suffix=B < $FN ; } ; done | sort -r -k 2 -h) ; clear ; echo "$D" | head -n 30 ; done
}

${@:-main}
