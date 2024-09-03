#!/usr/bin/env bash

determine_cri_bin() {
    if [ "${KUBEVIRT_CRI}" = "podman" ]; then
        echo podman
    elif [ "${KUBEVIRT_CRI}" = "docker" ]; then
        echo docker
    else
        if podman ps >/dev/null 2>&1; then
            echo podman
        elif docker ps >/dev/null 2>&1; then
            echo docker
        else
            echo ""
        fi
    fi
}

fail_if_cri_bin_missing() {
    if [ -z "${KUBEVIRT_CRI}" ]; then
        echo >&2 "no working container runtime found. Neither docker nor podman seems to work."
        exit 1
    fi
}

KUBEVIRT_CRI="$(determine_cri_bin)"
WASP_AGENT_DIR="$(
    cd "$(dirname "$BASH_SOURCE[0]")/../"
    pwd
)"
OUT_DIR=$WASP_AGENT_DIR/_out
ARTIFACTS=${ARTIFACTS:-${OUT_DIR}/artifacts}

# Use this environment variable to set a local path to a custom CA certificate for
# a private HTTPS docker registry. The intention is that this will be merged with the trust
# store in the build environment.

DOCKER_CA_CERT_FILE="${DOCKER_CA_CERT_FILE:-}"
DOCKERIZED_CUSTOM_CA_PATH="/etc/pki/ca-trust/source/anchors/custom-ca.crt"
