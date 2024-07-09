#!/usr/bin/bash

echo "WASP SWAP hook"

set -x

CG_PATH=$(jq -er '.linux.cgroupsPath' < config.json)
POD_NAME=$(jq -er '.annotations["io.kubernetes.pod.name"]' < config.json)

if [[ "$CG_PATH" =~ .*"burst".* && "$POD_NAME" =~ "virt-launcher".* ]];
then
  CONTAINERID=$(jq -er '.linux.cgroupsPath | split(":")[2]' < config.json)
  runc update $CONTAINERID --memory-swap -1
fi