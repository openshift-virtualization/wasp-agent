FROM quay.io/centos/centos:stream9

ENV GIMME_GO_VERSION=1.22.6
ENV OPERATOR_COURIER_VERSION=2.1.11
ENV GOLANGCI_LINT_VERSION=v1.60.1

# Install packages
RUN dnf install -y dnf-plugins-core && \
    dnf config-manager --enable crb && \
    dnf install -y --setopt=install_weak_deps=False \
        cpio \
        patch \
        make \
        git \
        sudo \
        gcc \
        gcc-c++ \
        glibc-static \
        libstdc++-static \
        glibc-devel \
        findutils \
        rsync-daemon \
        rsync \
        protobuf-compiler \
        python3 \
        python3-devel \
        python3-pip \
        python3-setuptools \
        redhat-rpm-config \
        jq \
        wget \
        rubygems \
        diffutils \
        skopeo && \
    dnf clean -y all

# Avoids the need to install sssd-client by disabling lookups
COPY nsswitch.conf /etc/nsswitch.conf

# reference to master is for an external repo and can't yet be changed
RUN mkdir -p /gimme && curl -sL \
    https://raw.githubusercontent.com/travis-ci/gimme/master/gimme | \
    HOME=/gimme bash >> /etc/profile.d/gimme.sh

ENV GOPATH="/go" GOBIN="/usr/bin" GO111MODULE="on"

# Install persistent go packages
RUN \
	source /etc/profile.d/gimme.sh && \
	eval $(go env) && \
	go install github.com/onsi/ginkgo/ginkgo@v1.14.1 && \
	go install golang.org/x/tools/cmd/goimports@latest && \
	go install mvdan.cc/sh/cmd/shfmt@latest && \
	go install github.com/mattn/goveralls@latest && \
	go install golang.org/x/lint/golint@latest && \
    go install -v github.com/golang/protobuf/protoc-gen-go@1643683 && \
	go install github.com/rmohr/go-swagger-utils/swagger-doc@latest && \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.11.3 && \
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0 && \
    rm -rf "${GOPATH}/pkg"

RUN set -x && \
    source /etc/profile.d/gimme.sh && \
    go install -v mvdan.cc/gofumpt@v0.6.0 && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOROOT)"/bin $GOLANGCI_LINT_VERSION

RUN pip3 install --upgrade operator-courier==${OPERATOR_COURIER_VERSION}

COPY rsyncd.conf /etc/rsyncd.conf

COPY entrypoint.sh /entrypoint.sh

# Add /root/go/src/github.com/openshift-virtualization/wasp-agent and mark it as a safe directory for git
RUN mkdir -p /root/go/src/github.com/openshift-virtualization/wasp-agent && \
    git config --global --add safe.directory /root/go/src/github.com/openshift-virtualization/wasp-agent

ENTRYPOINT [ "/entrypoint.sh" ]
