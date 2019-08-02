# syntax = docker/dockerfile-upstream:1.1.2-experimental

# Meta args applied to stage base names.

ARG TOOLS
ARG GO_VERSION

# The tools target provides base toolchain for the build.

FROM $TOOLS AS tools
ENV PATH /toolchain/bin
RUN ["/toolchain/bin/mkdir", "/bin", "/tmp"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/bin/bash", "/bin/sh"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/etc/ssl", "/etc/ssl"]
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /toolchain/bin v1.16.0

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
WORKDIR /osd
COPY ./internal/app/osd/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /trustd
COPY ./internal/app/trustd/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /machined
COPY ./internal/app/machined/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /proxyd
COPY ./internal/app/proxyd/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /ntpd
COPY ./internal/app/ntpd/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto

FROM scratch AS generate
COPY --from=generate-build /osd/proto/api.pb.go /internal/app/osd/proto/
COPY --from=generate-build /trustd/proto/api.pb.go /internal/app/trustd/proto/
COPY --from=generate-build /machined/proto/api.pb.go /internal/app/machined/proto/
COPY --from=generate-build /proxyd/proto/api.pb.go /internal/app/proxyd/proto/
COPY --from=generate-build /ntpd/proto/api.pb.go /internal/app/ntpd/proto/

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
COPY --from=generate /internal/app ./internal/app
RUN go list -mod=readonly all >/dev/null
RUN ! go mod tidy -v 2>&1 | grep .

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

# The proxyd target builds the proxyd image.

FROM base AS proxyd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/proxyd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /proxyd
RUN chmod +x /proxyd

FROM scratch AS proxyd
COPY --from=proxyd-build /proxyd /proxyd
ENTRYPOINT ["/proxyd"]

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
COPY --from=docker.io/autonomy/kernel:36cc240 /boot/vmlinuz /vmlinuz
COPY --from=docker.io/autonomy/kernel:36cc240 /boot/vmlinux /vmlinux

# The rootfs target provides the Talos rootfs.

FROM build AS rootfs-base
COPY --from=docker.io/autonomy/fhs:8467184 / /rootfs
COPY --from=docker.io/autonomy/ca-certificates:20f39f7 / /rootfs
COPY --from=docker.io/autonomy/containerd:03821f9 / /rootfs
COPY --from=docker.io/autonomy/cni:063e06f / /rootfs
COPY --from=docker.io/autonomy/dosfstools:767dee6 / /rootfs
COPY --from=docker.io/autonomy/eudev:05186a8 / /rootfs
COPY --from=docker.io/autonomy/iptables:a7aa58f / /rootfs
COPY --from=docker.io/autonomy/libressl:3fca2cf / /rootfs
COPY --from=docker.io/autonomy/libseccomp:80ea634 / /rootfs
COPY --from=docker.io/autonomy/musl:9bc7430 / /rootfs
COPY --from=docker.io/autonomy/runc:c79f79d / /rootfs
COPY --from=docker.io/autonomy/socat:032c783 / /rootfs
COPY --from=docker.io/autonomy/syslinux:85e1f9c / /rootfs
COPY --from=docker.io/autonomy/xfsprogs:5e50579 / /rootfs
COPY --from=docker.io/autonomy/kubeadm:6b75055 / /rootfs
COPY --from=docker.io/autonomy/crictl:ddbeea1 / /rootfs
COPY --from=docker.io/autonomy/base:f9a4941 /toolchain/lib/libblkid.* /rootfs/lib
COPY --from=docker.io/autonomy/base:f9a4941 /toolchain/lib/libuuid.* /rootfs/lib
COPY --from=docker.io/autonomy/base:f9a4941 /toolchain/lib/libkmod.* /rootfs/lib
COPY --from=docker.io/autonomy/kernel:36cc240 /lib/modules /rootfs/lib/modules
COPY --from=machined /machined /rootfs/sbin/init
COPY images/ntpd.tar /rootfs/usr/images/
COPY images/osd.tar /rootfs/usr/images/
COPY images/proxyd.tar /rootfs/usr/images/
COPY images/trustd.tar /rootfs/usr/images/
COPY images/networkd.tar /rootfs/usr/images/
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
COPY ./hack/cleanup.sh /toolchain/bin/cleanup.sh
RUN cleanup.sh /rootfs
RUN touch /rootfs/etc/resolv.conf
RUN touch /rootfs/etc/hosts
RUN touch /rootfs/etc/os-release
RUN mkdir -pv /rootfs/{boot,usr/local/share}
RUN mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni,usr/libexec/kubernetes}
RUN ln -s /etc/ssl /rootfs/etc/pki
RUN ln -s /etc/ssl /rootfs/usr/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/etc/ca-certificates

FROM rootfs-base AS rootfs-squashfs
COPY --from=rootfs / /rootfs
RUN mksquashfs /rootfs /rootfs.sqsh -all-root -noappend -comp xz -Xdict-size 100% -no-progress

FROM scratch AS rootfs
COPY --from=rootfs-base /rootfs /

# The initramfs target provides the Talos initramfs image.

FROM build AS initramfs-archive
WORKDIR /initramfs
COPY --from=rootfs-squashfs /rootfs.sqsh .
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
RUN apk --update add \
    bash \
    cdrkit \
    curl \
    qemu-img \
    syslinux \
    unzip \
    util-linux \
    xfsprogs
COPY --from=hashicorp/packer:1.4.2 /bin/packer /bin/packer
COPY hack/installer/packer.json /packer.json
COPY hack/installer/entrypoint.sh /bin/entrypoint.sh
COPY --from=kernel /vmlinuz /usr/install/vmlinuz
COPY --from=rootfs /usr/lib/syslinux/ /usr/lib/syslinux
COPY --from=initramfs /initramfs.xz /usr/install/initramfs.xz
COPY --from=osctl-linux-build /osctl-linux-amd64 /bin/osctl
ARG TAG
ENV VERSION ${TAG}
ENTRYPOINT ["entrypoint.sh"]

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
RUN --mount=type=cache,target=/.cache/go-build go test -v -race ${TESTPKGS}

# The lint target performs linting on the source code.

FROM base AS lint
COPY hack/golang/golangci-lint.yaml .
RUN --mount=type=cache,target=/.cache/go-build golangci-lint run --config golangci-lint.yaml

# The markdownlint target performs linting on Markdown files.

FROM node:8.16.1-alpine AS markdownlint
RUN npm install -g markdownlint-cli
RUN npm i sentences-per-line
WORKDIR /src
COPY .markdownlint.json .
COPY docs .
RUN markdownlint --rules /node_modules/sentences-per-line/index.js .
