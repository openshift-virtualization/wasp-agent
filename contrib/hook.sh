#!/usr/bin/bash

echo "WASP SWAP hook"

set -x

CGROUPROOT="/sys/fs/cgroup/kubepods.slice"
KUBEPOD_SLICE=$(jq -er '.linux.cgroupsPath | split(":")[0]' < config.json)
CONTAINERID=$(jq -er '.linux.cgroupsPath | split(":")[2]' < config.json)
CG_FS_PATH="$CGROUPROOT/$KUBEPOD_SLICE/crio-$CONTAINERID.scope"
if [[ $KUBEPOD_SLICE =~ .*besteffort-.* ]]; then
  CG_FS_PATH="$CGROUPROOT/kubepods-besteffort.slice/$KUBEPOD_SLICE/crio-$CONTAINERID.scope"
elif [[ $KUBEPOD_SLICE =~ .*burstable-.* ]]; then
  CG_FS_PATH="$CGROUPROOT/kubepods-burstable.slice/$KUBEPOD_SLICE/crio-$CONTAINERID.scope"
fi
echo 0 > $CG_FS_PATH/memory.swap.max
