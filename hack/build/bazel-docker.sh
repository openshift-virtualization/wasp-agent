#!/usr/bin/env bash

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

set -e
script_dir="$(cd "$(dirname "$0")" && pwd -P)"
source "${script_dir}"/common.sh
source "${script_dir}"/config.sh

mkdir -p "${WASP_DIR}/_out"

# update this whenever new builder tag is created
BUILDER_IMAGE=${BUILDER_IMAGE:-quay.io/bmordeha/kubevirt-wasp-bazel-builder:2407031059-d673c1a}

BUILDER_VOLUME="kubevirt-wasp-volume"
DOCKER_CA_CERT_FILE="${DOCKER_CA_CERT_FILE:-}"
DOCKERIZED_CUSTOM_CA_PATH="/etc/pki/ca-trust/source/anchors/custom-ca.crt"

DISABLE_SECCOMP=${DISABLE_SECCOMP:-}

SYNC_OUT=${SYNC_OUT:-true}
SYNC_VENDOR=${SYNC_VENDOR:-true}

# Create the persistent docker volume
if [ -z "$(${WASP_CRI} volume list | grep ${BUILDER_VOLUME})" ]; then
    ${WASP_CRI} volume create ${BUILDER_VOLUME}
fi

# Make sure that the output directory exists
echo "Making sure output directory exists..."
${WASP_CRI} run -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label=disable $DISABLE_SECCOMP --rm --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} mkdir -p /root/go/src/kubevirt.io/wasp-agent/_out

${WASP_CRI} run -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label=disable $DISABLE_SECCOMP --rm --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} git config --global --add safe.directory /root/go/src/kubevirt.io/wasp-agent
echo "Starting rsyncd"
# Start an rsyncd instance and make sure it gets stopped after the script exits
RSYNC_CID_WASP=$(${WASP_CRI} run -d -v "${BUILDER_VOLUME}:/root:rw,z" --security-opt label=disable $DISABLE_SECCOMP --cap-add SYS_CHROOT --expose 873 -P --entrypoint "/entrypoint-bazel.sh" ${BUILDER_IMAGE} /usr/bin/rsync --no-detach --daemon --verbose)

function finish() {
    ${WASP_CRI} stop --time 1 ${RSYNC_CID_WASP} >/dev/null 2>&1
    ${WASP_CRI} rm -f ${RSYNC_CID_WASP} >/dev/null 2>&1
}
trap finish EXIT

RSYNCD_PORT=$(${WASP_CRI} port $RSYNC_CID_WASP | cut -d':' -f2)

rsynch_fail_count=0

while ! rsync ${WASP_DIR}/ "rsync://root@127.0.0.1:${RSYNCD_PORT}/build/" &>/dev/null; do
    if [[ "$rsynch_fail_count" -eq 0 ]]; then
        printf "Waiting for rsyncd to be ready"
        sleep .1
    elif [[ "$rsynch_fail_count" -lt 30 ]]; then
        printf "."
        sleep 1
    else
        printf "failed"
        break
    fi
    rsynch_fail_count=$((rsynch_fail_count + 1))
done

printf "\n"

rsynch_fail_count=0

_rsync() {
    rsync -al "$@"
}

echo "Rsyncing ${WASP_DIR} to container"

# Copy wasp into the persistent docker volume
_rsync \
    --delete \
    --exclude 'cluster-up/cluster/**/.kubectl' \
    --exclude 'cluster-up/cluster/**/.oc' \
    --exclude 'cluster-up/cluster/**/.kubeconfig' \
    --exclude ".vagrant" \
    ${WASP_DIR}/ \
    "rsync://root@127.0.0.1:${RSYNCD_PORT}/build"

volumes="-v ${BUILDER_VOLUME}:/root:rw,z"
# append .docker directory as volume
mkdir -p "${HOME}/.docker"
volumes="$volumes -v ${HOME}/.docker:/root/.docker:ro,z"

if [[ WASP_CRI = podman* ]] && [[ -f "${XDG_RUNTIME_DIR-}/containers/auth.json" ]]; then
    volumes="$volumes --mount type=bind,source=${XDG_RUNTIME_DIR-}/containers/auth.json,target=/root/.docker/config.json,readonly"
elif [[ -f "${HOME}/.docker/config.json" ]]; then
    volumes="$volumes --mount type=bind,source=${HOME}/.docker/config.json,target=/root/.docker/config.json,readonly"
fi

if [ "${CI}" = "true" ]; then
    mkdir -p "$HOME/containers"
    volumes="$volumes -v ${HOME}/containers:/root/containers:ro,z"
fi

if [ -n "$DOCKER_CA_CERT_FILE" ]; then
    volumes="$volumes -v ${DOCKER_CA_CERT_FILE}:${DOCKERIZED_CUSTOM_CA_PATH}:ro,z"
fi

# Run the command
test -t 1 && USE_TTY="-it"
if ! $WASP_CRI exec -w /root/go/src/github.com/openshift-virtualization/wasp-agent ${USE_TTY} ${RSYNC_CID} /entrypoint.sh "$@"; then
    # Copy the build output out of the container, make sure that _out exactly matches the build result
    if [ "$SYNC_OUT" = "true" ]; then
        _rsync --delete "rsync://root@127.0.0.1:${RSYNCD_PORT}/out" ${OUT_DIR}
    fi
    exit 1
fi

# Copy the whole wasp data out to get generated sources and formatting changes
_rsync \
    --exclude 'cluster-up/cluster/**/.kubectl' \
    --exclude 'cluster-up/cluster/**/.oc' \
    --exclude 'cluster-up/cluster/**/.kubeconfig' \
    --exclude "_out" \
    --exclude "bin" \
    --exclude "vendor" \
    --exclude ".vagrant" \
    --exclude ".git" \
    "rsync://root@127.0.0.1:${RSYNCD_PORT}/build" \
    ${WASP_DIR}/

if [ "$SYNC_VENDOR" = "true" ] && [ -n $VENDOR_DIR ]; then
    _rsync --delete "rsync://root@127.0.0.1:${RSYNCD_PORT}/vendor" "${VENDOR_DIR}/"
fi


# Copy the build output out of the container, make sure that _out exactly matches the build result
if [ "$SYNC_OUT" = "true" ]; then
    _rsync --delete "rsync://root@127.0.0.1:${RSYNCD_PORT}/out" ${OUT_DIR}
fi