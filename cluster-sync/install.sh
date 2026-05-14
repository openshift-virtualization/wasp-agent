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

function install_kubevirt() {

  #Check if kubevrit is already installed
  if kubectl get crd kubevirts.kubevirt.io >> /dev/null 2>&1 ; then
    echo "Bypassing Kubevirt deployment since it's already installed"
    return
  fi

  if [ "$KUBEVIRT_RELEASE" = "latest_nightly" ]; then
    LATEST=$(curl -L https://storage.googleapis.com/kubevirt-prow/devel/nightly/release/kubevirt/kubevirt/latest)
    kubectl apply -f https://storage.googleapis.com/kubevirt-prow/devel/nightly/release/kubevirt/kubevirt/${LATEST}/kubevirt-operator.yaml
    kubectl apply -f https://storage.googleapis.com/kubevirt-prow/devel/nightly/release/kubevirt/kubevirt/${LATEST}/kubevirt-cr.yaml
  elif [ "$KUBEVIRT_RELEASE" = "latest_stable" ]; then
    RELEASE=$(curl https://storage.googleapis.com/kubevirt-prow/release/kubevirt/kubevirt/stable.txt)
    kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${RELEASE}/kubevirt-operator.yaml
    kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${RELEASE}/kubevirt-cr.yaml
  else
    kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_RELEASE}/kubevirt-operator.yaml
    kubectl apply -f https://github.com/kubevirt/kubevirt/releases/download/${KUBEVIRT_RELEASE}/kubevirt-cr.yaml
  fi
  # Ensure the KubeVirt CRD is created
  count=0
  until kubectl get crd kubevirts.kubevirt.io; do
      ((count++)) && ((count == 30)) && echo "KubeVirt CRD not found" && exit 1
      echo "waiting for KubeVirt CRD"
      sleep 1
  done

  # Ensure the KubeVirt API is available
  count=0
  until kubectl api-resources --api-group=kubevirt.io | grep kubevirts; do
      ((count++)) && ((count == 30)) && echo "KubeVirt API not found" && exit 1
      echo "waiting for KubeVirt API"
      sleep 1
  done


  # Ensure the KubeVirt CR is created
  count=0
  until kubectl -n kubevirt get kv kubevirt; do
      ((count++)) && ((count == 30)) && echo "KubeVirt CR not found" && exit 1
      echo "waiting for KubeVirt CR"
      sleep 1
  done

  # Wait until KubeVirt is ready
  count=0
  until kubectl wait -n kubevirt kv kubevirt --for condition=Available --timeout 5m; do
      ((count++)) && ((count == 5)) && echo "KubeVirt not ready in time" && exit 1
      echo "Error waiting for KubeVirt to be Available, sleeping 1m and retrying"
      sleep 1m
  done
}