#!/usr/bin/bash

# Expected to be set by DS
FSROOT=${FSROOT:-/host}

tune_system_slice() {
  [[ -n "$SWAPPINESS" ]] && ( set -x ; sysctl -w vm.swappiness=$SWAPPINESS ; )

  #
  echo "Tuning system.slice"
  #
  # Disable swap for system.slice and
  # set latency target to protect the root slice from io trash
  MAJMIN=$(findmnt $FSROOT/ --output MAJ:MIN -n | sed "s/:.*/:0/")  # fixme can be manually provided
  echo "Using MAJMIN $MAJMIN"
  ( se -x ; systemctl set-property --runtime system.slice MemorySwapMax=0 IODeviceLatencyTargetSec=$MAJMIN 50ms ; )

  #
  echo "Tune kubepods.slice"
  #
  # Gi is pow2
  {
    MEM_MAX=$(< $FSROOT/sys/fs/cgroup/kubepods.slice/memory.max)

    # We need to get this from kubelet.conf
    THRESHOLD_BYTES=$(numfmt --from=auto <<<100M)
    KUBELET_SOFT_MEM=$(jq -r ".evictionSoft[\"memory.available\"]" < $FSROOT/etc/kubernetes/kubelet.conf)
    if [[ "$KUBELET_SOFT_MEM" != "null" ]];
    then
      THRESHOLD_BYTES=$(numfmt --from=auto <<<$KUBELET_SOFT_MEM)
      echo "Aligning to soft-eviction threshold: $THRESHOLD_BYTES"
    else
      echo "Setting memory.high based on default $THRESHOLD_BYTES"
    fi

    MEM_HIGH=$(( MEM_MAX - THRESHOLD_BYTES ))
    ( set -x ; systemctl set-property --runtime kubepods.slice MemoryHigh=$MEM_HIGH ; )
  }
}

install_oci_hook() {
  # FIXME we shoud set noswap for all cgroups, not just leaves, just to be sure - but how?
  echo "installing hook"

  cp -v hook.sh $FSROOT/opt/oci-hook-swap.sh
  cp -v hook.json $FSROOT/run/containers/oci/hooks.d/swap-for-burstable.json
}

main() {
  if [[ ! -n "$DRY_RUN" ]]; then
    # FIXME hardlinks are broken if FSROOT is used, but we need it
    [[ ! -d /run/containers ]] && ln -s $FSROOT/run/containers /run/containers

    tune_system_slice
    install_oci_hook
  fi
  echo "Done"

  sleep inf
}

swaptop() {
  while sleep 0.5 ; do D=$(uptime ; free -m ; find /sys/fs/cgroup -name memory.swap.current | while read FN ; do [[ -f "$FN" && "$(cat $FN)" -gt 0 ]] && { echo -n "$FN " ; numfmt --to=iec-i --suffix=B < $FN ; } ; done | sort -r -k 2 -h) ; clear ; echo "$D" | head -n 30 ; done
}

${@:-main}
