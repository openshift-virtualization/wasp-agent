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
RUN set -x && \
    mkdir -p /go && \
    source /etc/profile.d/gimme.sh && \
    go install -v golang.org/x/tools/cmd/goimports@d5fe738 && \
    go install -v mvdan.cc/sh/v3/cmd/shfmt@v3.1.1 && \
    go install -v k8s.io/code-generator/cmd/conversion-gen@v0.20.2  && \
    go install -v k8s.io/code-generator/cmd/deepcopy-gen@v0.20.2 && \
    go install -v k8s.io/code-generator/cmd/defaulter-gen@v0.20.2  && \
    go install -v k8s.io/kube-openapi/cmd/openapi-gen@f5883ff && \
    go install -v github.com/golang/protobuf/protoc-gen-go@1643683 && \
    go install -v k8s.io/code-generator/cmd/client-gen@v0.20.2 && \
    go install -v github.com/securego/gosec/v2/cmd/gosec@0ce48a5 && \
    go install -v sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0 && \
    go install -v github.com/kubevirt/monitoring/monitoringlinter/cmd/monitoringlinter@e2be790 && \
    go clean -cache -modcache

RUN set -x && \
    source /etc/profile.d/gimme.sh && \
    go install -v github.com/mattn/goveralls@21feffdfd && \
    go install -v github.com/rmohr/mock/mockgen@v0.0.0-20170327080805-5980f5d9a70d && \
    go install -v github.com/rmohr/go-swagger-utils/swagger-doc@v0.0.0-20181110145135-878f7deb1d17 && \
    go clean -cache -modcache

RUN set -x && \
    source /etc/profile.d/gimme.sh && \
    git clone https://github.com/kubernetes/test-infra.git && \
    cd /test-infra && \
    git checkout f2693aba912dd40c974304caca999d45ee8dce33 && \
    cd /test-infra/robots/pr-creator && \
    go install && \
    cd /test-infra/robots/issue-creator && \
    go install && \
    cd /test-infra/robots/pr-labeler && \
    go install && \
    go clean -cache -modcache -r && \
    rm -rf /test-infra && \
    rm -rf /go && mkdir /go

RUN set -x && \
    source /etc/profile.d/gimme.sh && \
    go install -v mvdan.cc/gofumpt@v0.6.0 && \
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "$(go env GOROOT)"/bin $GOLANGCI_LINT_VERSION

RUN pip3 install --upgrade operator-courier==${OPERATOR_COURIER_VERSION}

COPY rsyncd.conf /etc/rsyncd.conf

COPY entrypoint.sh /entrypoint.sh

# Add /root/go/src/kubevirt.io/kubevirt and mark it as a safe directory for git
RUN mkdir -p /root/go/src/kubevirt.io/kubevirt && git config --global --add safe.directory /root/go/src/kubevirt.io/kubevirt

ENTRYPOINT [ "/entrypoint.sh" ]
