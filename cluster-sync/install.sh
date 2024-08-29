#!/usr/bin/env bash

set -e

function install_wasp {
  if _kubectl get crd securitycontextconstraints.security.openshift.io &> /dev/null; then
    _kubectl apply -f "./_out/manifests/release/wasp.yaml"
  else
    filtered_yaml=$(awk '/^allowHostDirVolumePlugin: true$/,/^---/{next}1' "./_out/manifests/release/wasp.yaml")
    echo "$filtered_yaml" | _kubectl apply -f -
  fi
}

function delete_wasp {
  if [ -f "./_out/manifests/release/wasp.yaml" ]; then
    if _kubectl get crd securitycontextconstraints.security.openshift.io &> /dev/null; then
      _kubectl delete --ignore-not-found -f "./_out/manifests/release/wasp.yaml"
    else
      filtered_yaml=$(awk '/^allowHostDirVolumePlugin: true$/,/^---/{next}1' "./_out/manifests/release/wasp.yaml")
      echo "$filtered_yaml" | _kubectl delete --ignore-not-found -f -
    fi
  else
    echo "File ./_out/manifests/release/wasp.yaml does not exist."
  fi
}