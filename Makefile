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

.PHONY: manifests \
		cluster-up cluster-down cluster-sync \
		test test-functional test-unit test-lint \
		publish \
		wasp \
		fmt \
		goveralls \
		release-description \
		bazel-build-images push-images \
		fossa
all: build

build:  wasp manifest-generator

DOCKER?=1
ifeq (${DOCKER}, 1)
	# use entrypoint.sh (default) as your entrypoint into the container
	DO=./hack/build/in-docker.sh
	# use entrypoint-bazel.sh as your entrypoint into the container.
	DO_BAZ=./hack/build/bazel-docker.sh
else
	DO=eval
	DO_BAZ=eval
endif

ifeq ($(origin KUBEVIRT_RELEASE), undefined)
	KUBEVIRT_RELEASE="latest_nightly"
endif

all: manifests build-images

manifests:
	${DO_BAZ} "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} VERBOSITY=${VERBOSITY} PULL_POLICY=${PULL_POLICY} CR_NAME=${CR_NAME} WASP_NAMESPACE=${WASP_NAMESPACE} MAX_AVERAGE_SWAPIN_PAGES_PER_SECOND=${MAX_AVERAGE_SWAPIN_PAGES_PER_SECOND} MAX_AVERAGE_SWAPOUT_PAGES_PER_SECOND=${MAX_AVERAGE_SWAPOUT_PAGES_PER_SECOND} SWAP_UTILIZATION_THRESHOLD_FACTOR=${SWAP_UTILIZATION_THRESHOLD_FACTOR} AVERAGE_WINDOW_SIZE_SECONDS=${AVERAGE_WINDOW_SIZE_SECONDS}  DEPLOY_PROMETHEUS_RULE=${DEPLOY_PROMETHEUS_RULE} ./hack/build/build-manifests.sh"

builder-push:
	./hack/build/bazel-build-builder.sh

cluster-up:
	eval "KUBEVIRT_RELEASE=${KUBEVIRT_RELEASE} KUBEVIRT_SWAP_ON=true ./cluster-up/up.sh"

cluster-down:
	./cluster-up/down.sh

push-images:
	eval "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG}  ./hack/build/build-docker.sh push"

build-images:
	eval "DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG}  ./hack/build/build-docker.sh"

push: build-images push-images

cluster-clean-wasp:
	./cluster-sync/clean.sh

cluster-sync: cluster-clean-wasp
	./cluster-sync/sync.sh WASP_AVAILABLE_TIMEOUT=${WASP_AVAILABLE_TIMEOUT} DOCKER_PREFIX=${DOCKER_PREFIX} DOCKER_TAG=${DOCKER_TAG} PULL_POLICY=${PULL_POLICY} WASP_NAMESPACE=${WASP_NAMESPACE}

test: WHAT = ./pkg/... ./cmd/...
test: bootstrap-ginkgo
	${DO_BAZ} "ACK_GINKGO_DEPRECATIONS=${ACK_GINKGO_DEPRECATIONS} ./hack/build/run-unit-tests.sh ${WHAT}"

build-functest:
	${DO_BAZ} ./hack/build/build-functest.sh

functest:  WHAT = ./tests/...
functest: build-functest
	./hack/build/run-functional-tests.sh ${WHAT} "${TEST_ARGS}"

bootstrap-ginkgo:
	${DO_BAZ} ./hack/build/bootstrap-ginkgo.sh

manifest-generator:
	GO111MODULE=${GO111MODULE:-off} go build -o manifest-generator -v tools/manifest-generator/*.go
wasp:
	go build -o wasp -v cmd/wasp/*.go
	chmod 777 wasp

release-description:
	./hack/build/release-description.sh ${RELREF} ${PREREF}

clean:
	rm ./wasp -f

fmt:
	go fmt .

run: build
	sudo ./wasp
