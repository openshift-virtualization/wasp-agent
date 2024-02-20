#!/usr/bin/bash

echo "WASP SWAP hook"

set -x

CG_PATH=$(cat config.json | jq -r '.linux.cgroupsPath')
CONTAINERID=$(echo "$CG_PATH" | cut -d: -f3)

[[ "$CG_PATH" = *burst* ]] && echo max > /sys/fs/cgroup/kubepods.slice/*burst*/*/crio-$CONTAINERID*/memory.swap.max || :
