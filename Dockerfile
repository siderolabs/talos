# syntax = docker/dockerfile-upstream:1.1.2-experimental

# Meta args applied to stage base names.

ARG TOOLS
ARG GO_VERSION

# The tools target provides base toolchain for the build.

FROM $TOOLS AS tools
ENV PATH /toolchain/bin:/toolchain/go/bin
RUN ["/toolchain/bin/mkdir", "/bin", "/tmp"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/bin/bash", "/bin/sh"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/etc/ssl", "/etc/ssl"]
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /toolchain/bin v1.21.0
RUN cd $(mktemp -d) \
    && go mod init tmp \
    && go get mvdan.cc/gofumpt/gofumports \
    && mv /go/bin/gofumports /toolchain/go/bin/gofumports
RUN curl -sfL https://github.com/uber/prototool/releases/download/v1.8.0/prototool-Linux-x86_64.tar.gz | tar -xz --strip-components=2 -C /toolchain/bin prototool/bin/prototool
COPY --from=autonomy/protoc-gen-proxy:4a9e5da /protoc-gen-proxy /toolchain/bin/protoc-gen-proxy
COPY ./hack/docgen /go/src/github.com/talos-systems/docgen
RUN cd /go/src/github.com/talos-systems/docgen \
    && go build . \
    && mv docgen /toolchain/go/bin/

# The build target creates a container that will be used to build Talos source
# code.

FROM scratch AS build
COPY --from=tools / /
SHELL ["/toolchain/bin/bash", "-c"]
ENV PATH /toolchain/bin:/toolchain/go/bin
ENV GO111MODULE on
ENV GOPROXY https://proxy.golang.org
ENV CGO_ENABLED 0
WORKDIR /src

# The generate target generates code from protobuf service definitions.

FROM build AS generate-build
# Common needs to be at or near the top to satisfy the subsequent imports
COPY ./api/common/common.proto /api/common/common.proto
RUN protoc -I/api --go_out=plugins=grpc:/api/common /api/common/common.proto
COPY ./api/os/os.proto /api/os/os.proto
RUN protoc -I/api --go_out=plugins=grpc:/api/os /api/os/os.proto
COPY ./api/security/security.proto /api/security/security.proto
RUN protoc -I/api --go_out=plugins=grpc:/api/security /api/security/security.proto
COPY ./api/machine/machine.proto /api/machine/machine.proto
RUN protoc -I/api --go_out=plugins=grpc:/api/machine /api/machine/machine.proto
COPY ./api/time/time.proto /api/time/time.proto
RUN protoc -I/api --go_out=plugins=grpc:/api/time /api/time/time.proto
COPY ./api/network/network.proto /api/network/network.proto
RUN protoc -I/api --go_out=plugins=grpc:/api/network /api/network/network.proto
# Genenrate api bits last so we have other proto files in place to satisfy the import
COPY ./api/api.proto /api/api.proto
RUN protoc -I/api --plugin=proxy --proxy_out=plugins=grpc+proxy:/api /api/api.proto
# Gofumports generated files to adjust import order
RUN gofumports -w -local github.com/talos-systems/talos /api/

FROM scratch AS generate
COPY --from=generate-build /api/common/github.com/talos-systems/talos/api/common/common.pb.go /api/common/
COPY --from=generate-build /api/github.com/talos-systems/talos/api/api.pb.go /api/
COPY --from=generate-build /api/os/github.com/talos-systems/talos/api/os/os.pb.go /api/os/
COPY --from=generate-build /api/security/github.com/talos-systems/talos/api/security/security.pb.go /api/security/
COPY --from=generate-build /api/machine/github.com/talos-systems/talos/api/machine/machine.pb.go /api/machine/
COPY --from=generate-build /api/time/github.com/talos-systems/talos/api/time/time.pb.go /api/time/
COPY --from=generate-build /api/network/github.com/talos-systems/talos/api/network/network.pb.go /api/network/

# The base target provides a container that can be used to build all Talos
# assets.

FROM build AS base
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download
RUN go mod verify
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY --from=generate /api ./api
RUN go list -mod=readonly all >/dev/null
RUN ! go mod tidy -v 2>&1 | grep .

FROM base AS docs-build
RUN go generate ./pkg/config/types/v1alpha1

FROM scratch AS docs
COPY --from=docs-build /tmp/v1alpha1.md /docs/website/content/v0.3/en/configuration/v1alpha1.md

# The init target builds the init binary.

FROM base AS init-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/init
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /init
RUN chmod +x /init

FROM scratch AS init
COPY --from=init-build /init /init

# The machined target builds the machined image.

FROM base AS machined-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/machined
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /machined
RUN chmod +x /machined

FROM scratch AS machined
COPY --from=machined-build /machined /machined

# The ntpd target builds the ntpd image.

FROM base AS ntpd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/ntpd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /ntpd
RUN chmod +x /ntpd

FROM scratch AS ntpd
COPY --from=ntpd-build /ntpd /ntpd
ENTRYPOINT ["/ntpd"]

# The apid target builds the api image.

FROM base AS apid-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/apid
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /apid
RUN chmod +x /apid

FROM scratch AS apid
COPY --from=apid-build /apid /apid
ENTRYPOINT ["/apid"]

# The osd target builds the osd image.

FROM base AS osd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/osd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osd
RUN chmod +x /osd

FROM scratch AS osd
COPY --from=osd-build /osd /osd
ENTRYPOINT ["/osd"]

# The trustd target builds the trustd image.

FROM base AS trustd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/trustd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /trustd
RUN chmod +x /trustd

FROM scratch AS trustd
COPY --from=trustd-build /trustd /trustd
ENTRYPOINT ["/trustd"]

# The networkd target builds the networkd image.

FROM base AS networkd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/networkd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /networkd
RUN chmod +x /networkd

FROM scratch AS networkd
COPY --from=networkd-build /networkd /networkd
ENTRYPOINT ["/networkd"]

# The osctl targets build the osctl binaries.

FROM base AS osctl-linux-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/cmd/osctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-linux-amd64
RUN chmod +x /osctl-linux-amd64

FROM scratch AS osctl-linux
COPY --from=osctl-linux-build /osctl-linux-amd64 /osctl-linux-amd64

FROM base AS osctl-darwin-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/cmd/osctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-darwin-amd64
RUN chmod +x /osctl-darwin-amd64

FROM scratch AS osctl-darwin
COPY --from=osctl-darwin-build /osctl-darwin-amd64 /osctl-darwin-amd64

# The kernel target is the linux kernel.

FROM scratch AS kernel
COPY --from=docker.io/autonomy/kernel:89798b3 /boot/vmlinuz /vmlinuz
COPY --from=docker.io/autonomy/kernel:89798b3 /boot/vmlinux /vmlinux

# The rootfs target provides the Talos rootfs.

FROM build AS rootfs-base
COPY --from=docker.io/autonomy/fhs:89798b3 / /rootfs
COPY --from=docker.io/autonomy/ca-certificates:89798b3 / /rootfs
COPY --from=docker.io/autonomy/containerd:89798b3 / /rootfs
COPY --from=docker.io/autonomy/dosfstools:89798b3 / /rootfs
COPY --from=docker.io/autonomy/eudev:89798b3 / /rootfs
COPY --from=docker.io/autonomy/iptables:89798b3 / /rootfs
COPY --from=docker.io/autonomy/libressl:89798b3 / /rootfs
COPY --from=docker.io/autonomy/libseccomp:89798b3 / /rootfs
COPY --from=docker.io/autonomy/musl:89798b3 / /rootfs
COPY --from=docker.io/autonomy/runc:89798b3 / /rootfs
COPY --from=docker.io/autonomy/socat:89798b3 / /rootfs
COPY --from=docker.io/autonomy/syslinux:89798b3 / /rootfs
COPY --from=docker.io/autonomy/xfsprogs:89798b3 / /rootfs
COPY --from=docker.io/autonomy/util-linux:89798b3 /lib/libblkid.* /rootfs/lib
COPY --from=docker.io/autonomy/util-linux:89798b3 /lib/libuuid.* /rootfs/lib
COPY --from=docker.io/autonomy/kmod:89798b3 /usr/lib/libkmod.* /rootfs/lib
COPY --from=docker.io/autonomy/kernel:89798b3 /lib/modules /rootfs/lib/modules
COPY --from=machined /machined /rootfs/sbin/init
COPY images/apid.tar /rootfs/usr/images/
COPY images/ntpd.tar /rootfs/usr/images/
COPY images/osd.tar /rootfs/usr/images/
COPY images/trustd.tar /rootfs/usr/images/
COPY images/networkd.tar /rootfs/usr/images/
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
COPY ./hack/cleanup.sh /toolchain/bin/cleanup.sh
RUN cleanup.sh /rootfs
COPY hack/containerd.toml /etc/containerd.toml
COPY hack/containerd.toml /etc/containerd-system.toml
RUN touch /rootfs/etc/resolv.conf
RUN touch /rootfs/etc/hosts
RUN touch /rootfs/etc/os-release
RUN mkdir -pv /rootfs/{boot,usr/local/share,mnt}
RUN mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni,usr/libexec/kubernetes}
RUN ln -s /etc/ssl /rootfs/etc/pki
RUN ln -s /etc/ssl /rootfs/usr/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/etc/ca-certificates

FROM rootfs-base AS rootfs-squashfs
COPY --from=rootfs / /rootfs
RUN mksquashfs /rootfs /rootfs.sqsh -all-root -noappend -comp xz -Xdict-size 100% -no-progress

FROM scratch AS squashfs
COPY --from=rootfs-squashfs /rootfs.sqsh /

FROM scratch AS rootfs
COPY --from=rootfs-base /rootfs /

# The initramfs target provides the Talos initramfs image.

FROM build AS initramfs-archive
WORKDIR /initramfs
COPY --from=squashfs /rootfs.sqsh .
COPY --from=init /init .
RUN set -o pipefail && find . 2>/dev/null | cpio -H newc -o | xz -v -C crc32 -0 -e -T 0 -z >/initramfs.xz

FROM scratch AS initramfs
COPY --from=initramfs-archive /initramfs.xz /initramfs.xz

# The container target generates a docker image that can be used to run Talos
# in containers.

FROM scratch AS container
COPY --from=rootfs / /
ENTRYPOINT ["/sbin/init"]

# The installer target generates an image that can be used to install Talos to
# various environments.

FROM alpine:3.8 AS installer
RUN apk add --no-cache --update \
    bash \
    ca-certificates \
    cdrkit \
    qemu-img \
    syslinux \
    util-linux \
    xfsprogs
COPY hack/installer/entrypoint.sh /bin/entrypoint.sh
COPY hack/installer/template.ovf /template.ovf
COPY --from=kernel /vmlinuz /usr/install/vmlinuz
COPY --from=rootfs /usr/lib/syslinux/ /usr/lib/syslinux
COPY --from=initramfs /initramfs.xz /usr/install/initramfs.xz
COPY --from=osctl-linux-build /osctl-linux-amd64 /bin/osctl
ARG TAG
ENV VERSION ${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
ENTRYPOINT ["entrypoint.sh"]
ONBUILD RUN apk add --no-cache --update \
    cpio \
    squashfs-tools \
    xz
ONBUILD WORKDIR /initramfs
ONBUILD ARG RM
ONBUILD RUN xz -d /usr/install/initramfs.xz \
    && cpio -idvm < /usr/install/initramfs \
    && unsquashfs -f -d /rootfs rootfs.sqsh \
    && for f in ${RM}; do rm -rfv /rootfs$f; done \
    && rm /usr/install/initramfs \
    && rm rootfs.sqsh
ONBUILD COPY --from=customization / /rootfs
ONBUILD RUN find /rootfs \
    && mksquashfs /rootfs rootfs.sqsh -all-root -noappend -comp xz -Xdict-size 100% -no-progress \
    && set -o pipefail && find . 2>/dev/null | cpio -H newc -o | xz -v -C crc32 -0 -e -T 0 -z >/usr/install/initramfs.xz \
    && rm -rf /rootfs \
    && rm -rf /initramfs
ONBUILD WORKDIR /

# The test target performs tests on the source code.

FROM base AS unit-tests-runner
RUN unlink /etc/ssl
COPY --from=rootfs / /
COPY hack/golang/test.sh /bin
ARG TESTPKGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache/go-build /bin/test.sh ${TESTPKGS}
FROM scratch AS unit-tests
COPY --from=unit-tests-runner /src/coverage.txt /coverage.txt

# The unit-tests-race target performs tests with race detector.

FROM golang:${GO_VERSION} AS unit-tests-race
COPY --from=base /src /src
COPY --from=base /go/pkg/mod /go/pkg/mod
WORKDIR /src
ENV GO111MODULE on
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build go test -v -count 1 -race ${TESTPKGS}

# The integration-test target builds integration test binary.

FROM base AS integration-test-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go test -c \
    -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test
COPY --from=integration-test-build /src/integration.test /integration-test

# The lint target performs linting on the source code.

FROM base AS lint
COPY hack/golang/golangci-lint.yaml .
ENV GOGC=50
RUN --mount=type=cache,target=/.cache/go-build golangci-lint run --config golangci-lint.yaml
RUN find . -name '*.pb.go' | xargs rm
RUN FILES="$(gofumports -l -local github.com/talos-systems/talos .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumports -w -local github.com/talos-systems/talos .':\n${FILES}"; exit 1)

# The protolint target performs linting on Markdown files.

FROM base AS protolint
COPY prototool.yaml /src
RUN prototool lint --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include .

# The markdownlint target performs linting on Markdown files.

FROM node:8.16.1-alpine AS markdownlint
RUN npm install -g markdownlint-cli
RUN npm i sentences-per-line
WORKDIR /src
COPY .markdownlint.json .
COPY docs .
RUN markdownlint --rules /node_modules/sentences-per-line/index.js .
