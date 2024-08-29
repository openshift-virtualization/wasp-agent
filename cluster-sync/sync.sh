#!/bin/bash -e

wasp=$1
wasp="${wasp##*/}"

echo wasp

source ./hack/build/config.sh
source ./hack/build/common.sh
source ./cluster-up/hack/common.sh
source ./cluster-up/cluster/${KUBEVIRT_PROVIDER}/provider.sh

if [ "${KUBEVIRT_PROVIDER}" = "external" ]; then
   WASP_SYNC_PROVIDER="external"
else
   WASP_SYNC_PROVIDER="kubevirtci"
fi
source ./cluster-sync/${WASP_SYNC_PROVIDER}/provider.sh


WASP_NAMESPACE=${WASP_NAMESPACE:-wasp}
WASP_INSTALL_TIMEOUT=${WASP_INSTALL_TIMEOUT:-120}
WASP_AVAILABLE_TIMEOUT=${WASP_AVAILABLE_TIMEOUT:-600}

# Set controller verbosity to 3 for functional tests.
export VERBOSITY=3

PULL_POLICY=${PULL_POLICY:-IfNotPresent}
# The default DOCKER_PREFIX is set to kubevirt and used for builds, however we don't use that for cluster-sync
# instead we use a local registry; so here we'll check for anything != "external"
# wel also confuse this by swapping the setting of the DOCKER_PREFIX variable around based on it's context, for
# build and push it's localhost, but for manifests, we sneak in a change to point a registry container on the
# kubernetes cluster.  So, we introduced this MANIFEST_REGISTRY variable specifically to deal with that and not
# have to refactor/rewrite any of the code that works currently.
MANIFEST_REGISTRY=$DOCKER_PREFIX

if [ "${KUBEVIRT_PROVIDER}" != "external" ]; then
  registry=${IMAGE_REGISTRY:-localhost:$(_port registry)}
  DOCKER_PREFIX=${registry}
  MANIFEST_REGISTRY="registry:5000"
fi

if [ "${KUBEVIRT_PROVIDER}" == "external" ]; then
  # No kubevirtci local registry, likely using something external
  if [[ $(${WASP_CRI} login --help | grep authfile) ]]; then
    registry_provider=$(echo "$DOCKER_PREFIX" | cut -d '/' -f 1)
    echo "Please log in to "${registry_provider}", bazel push expects external registry creds to be in ~/.docker/config.json"
    ${WASP_CRI} login --authfile "${HOME}/.docker/config.json" $registry_provider
  fi
fi

# Need to set the DOCKER_PREFIX appropriately in the call to `make docker push`, otherwise make will just pass in the default `kubevirt`

DOCKER_PREFIX=$MANIFEST_REGISTRY PULL_POLICY=$PULL_POLICY make manifests
DOCKER_PREFIX=$DOCKER_PREFIX make push

function check_wasp_daemonset_pods() {
  # Get the number of desired pods and available pods
  desired_pods=$(kubectl get daemonset wasp-agent -n "$WASP_NAMESPACE" -o jsonpath='{.status.desiredNumberScheduled}')
  available_pods=$(kubectl get daemonset wasp-agent -n "$WASP_NAMESPACE" -o jsonpath='{.status.numberAvailable}')
  available_pods=${available_pods:-0}

  echo "Desired pods: $desired_pods"
  echo "Available pods: $available_pods"

  # Check if the number of desired pods matches the number of available pods
  if [ "$desired_pods" -eq "$available_pods" ] ; then
    return 0
  else
    return 1
  fi
}

function wait_wasp_available {
  retry_count="${WASP_INSTALL_TIMEOUT}"
  echo "Waiting for DaemonSet wasp-agent in namespace '$WASP_NAMESPACE' to be ready..."

  # Loop for the specified number of retries
  for ((i = 0; i < retry_count; i++)); do
    # Check if DaemonSet pods are ready
    if check_wasp_daemonset_pods ; then
      echo "All pods in DaemonSet wasp are available."
      exit 0
    fi

    # Wait for 1 second before retrying
    sleep 1
  done
    echo "Warning: wasp is not ready!"
}

mkdir -p ./_out/tests

# Install WASP
install_wasp

wait_wasp_available
