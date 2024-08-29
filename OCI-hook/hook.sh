#!/usr/bin/bash

echo "WASP SWAP hook"

set -x

CG_PATH=$(jq -er '.linux.cgroupsPath' < config.json)
POD_NAMESPACE=$(jq -er '.annotations["io.kubernetes.pod.namespace"]' < config.json)

if [[ "$CG_PATH" =~ .*"burst".* ]];
then
  CONTAINERID=$(jq -er '.linux.cgroupsPath | split(":")[2]' < config.json)
  runc update $CONTAINERID --memory-swap -1
fi
