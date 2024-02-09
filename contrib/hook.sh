#!/usr/bin/bash

echo "WASP SWAP hook"

set -x
date
env

CG_PATH=$(cat config.json | jq -r '.linux.cgroupsPath')
CONTAINERID=$(echo "$CG_PATH" | cut -d: -f3)

grep burst <<<$CG_PATH && echo max > /sys/fs/cgroup/kubepods.slice/*burst*/*/crio-$CONTAINERID*/memory.swap.max || :
