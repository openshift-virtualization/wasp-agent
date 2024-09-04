#Copyright 2023 The WASP Authors.
#
#Licensed under the Apache License, Version 2.0 (the "License");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an "AS IS" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
#!/usr/bin/env bash

determine_cri_bin() {
    if [ "${WASP_CRI-}" = "podman" ]; then
        echo podman
    elif [ "${WASP_CRI-}" = "docker" ]; then
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
    if [ -z "${WASP_CRI}" ]; then
        echo >&2 "no working container runtime found. Neither docker nor podman seems to work."
        exit 1
    fi
}

WASP_DIR="$(
    cd "$(dirname "$BASH_SOURCE[0]")/../../"
    pwd
)"
WASP_CRI="$(determine_cri_bin)"

# Use this environment variable to set a local path to a custom CA certificate for
# a private HTTPS docker registry. The intention is that this will be merged with the trust
# store in the build environment.

DOCKER_CA_CERT_FILE="${DOCKER_CA_CERT_FILE:-}"
DOCKERIZED_CUSTOM_CA_PATH="/etc/pki/ca-trust/source/anchors/custom-ca.crt"
BIN_DIR=${WASP_DIR}/bin
OUT_DIR=$WASP_DIR/_out
ARTIFACTS=${ARTIFACTS:-${OUT_DIR}/artifacts}
CMD_OUT_DIR=${WASP_DIR}/cmd
TESTS_OUT_DIR=${OUT_DIR}/tests
BUILD_DIR=${WASP_DIR}/hack/builder
MANIFEST_TEMPLATE_DIR=${WASP_DIR}/manifests/templates
MANIFEST_GENERATED_DIR=${WASP_DIR}/manifests/generated
CACHE_DIR=${OUT_DIR}/gocache
VENDOR_DIR=$WASP_DIR/vendor
ARCHITECTURE="${BUILD_ARCH:-$(uname -m)}"
HOST_ARCHITECTURE="$(uname -m)"