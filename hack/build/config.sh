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
WASP_IMAGE_NAME=${WASP_IMAGE_NAME:-wasp}

DOCKER_PREFIX=${DOCKER_PREFIX:-"quay.io/bmordeha"}
DOCKER_TAG=${DOCKER_TAG:-latest}
VERBOSITY=${VERBOSITY:-1}
PULL_POLICY=${PULL_POLICY:-Always}
WASP_NAMESPACE=${WASP_NAMESPACE:-wasp}
MAX_AVERAGE_SWAPIN_PAGES_PER_SECOND=${MAX_AVERAGE_SWAPIN_PAGES_PER_SECOND:-1000}
MAX_AVERAGE_SWAPOUT_PAGES_PER_SECOND=${MAX_AVERAGE_SWAPOUT_PAGES_PER_SECOND:-1000}
AVERAGE_WINDOW_SIZE_SECONDS=${AVERAGE_WINDOW_SIZE_SECONDS:-30}
SWAP_UTILIZATION_THRESHOLD_FACTOR=${SWAP_UTILIZATION_THRESHOLD_FACTOR:-0.8}
DEPLOY_PROMETHEUS_RULE=${DEPLOY_PROMETHEUS_RULE:-false}
CR_NAME=${CR_NAME:-wasp}

function parseTestOpts() {
    pkgs=""
    test_args=""
    while [[ $# -gt 0 ]] && [[ $1 != "" ]]; do
        case "${1}" in
        --test-args=*)
            test_args="${1#*=}"
            shift 1
            ;;
        ./*...)
            pkgs="${pkgs} ${1}"
            shift 1
            ;;
        *)
            echo "ABORT: Unrecognized option \"$1\""
            exit 1
            ;;
        esac
    done
}