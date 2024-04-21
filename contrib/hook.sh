#!/usr/bin/bash

echo "WASP SWAP hook"

set -x

CG_PATH=$(jq -er '.linux.cgroupsPath' < config.json)

if [[ "$CG_PATH" = *burst* ]];
then
  CONTAINERID=$(jq -er '.linux.cgroupsPath | split(":")[2]' < config.json)
  echo max > /sys/fs/cgroup/kubepods.slice/*burst*/*/crio-$CONTAINERID*/memory.swap.max
fi