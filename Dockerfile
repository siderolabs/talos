# syntax = docker/dockerfile:1.1-experimental

ARG TOOLS
FROM $TOOLS AS tools

# The build target creates a container that will be used to build Talos source
# code.

FROM scratch AS build
COPY --from=tools / /
SHELL [ "/toolchain/bin/bash", "-c" ]
ENV PATH /toolchain/bin:/toolchain/go/bin
ENV GO111MODULE on
ENV GOPROXY https://proxy.golang.org
ENV CGO_ENABLED 0
RUN mkdir /tmp
RUN ln -sv /toolchain/etc/ssl /etc/ssl
WORKDIR /src

# The generate target generates code from protobuf service definitions.

FROM build AS generate-build
WORKDIR /osd
COPY ./internal/app/osd/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /trustd
COPY ./internal/app/trustd/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /init
COPY ./internal/app/init/proto ./proto
RUN protoc -I./proto --go_out=plugins=grpc:proto proto/api.proto
FROM scratch AS generate
COPY --from=generate-build /osd/proto/api.pb.go /internal/app/osd/proto/
COPY --from=generate-build /trustd/proto/api.pb.go /internal/app/trustd/proto/
COPY --from=generate-build /init/proto/api.pb.go /internal/app/init/proto/

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
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/init
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /init
RUN chmod +x /init
FROM scratch AS init
COPY --from=init-build /init /init

# The ntpd target builds the ntpd image.

FROM base AS ntpd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/ntpd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /ntpd
RUN chmod +x /ntpd
FROM scratch AS ntpd
COPY --from=ntpd-build /ntpd /ntpd
ENTRYPOINT ["/ntpd"]

# The osd target builds the osd image.

FROM base AS osd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/osd
RUN --mount=type=cache,target=/root/.cache go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osd
RUN chmod +x /osd
FROM scratch AS osd
COPY --from=osd-build /osd /osd
ENTRYPOINT ["/osd"]

# The proxyd target builds the proxyd image.

FROM base AS proxyd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/proxyd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /proxyd
RUN chmod +x /proxyd
FROM scratch AS proxyd
COPY --from=proxyd-build /proxyd /proxyd
ENTRYPOINT ["/proxyd"]

# The trustd target builds the trustd image.

FROM base AS trustd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/trustd
RUN --mount=type=cache,target=/root/.cache go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /trustd
RUN chmod +x /trustd
FROM scratch AS trustd
COPY --from=trustd-build /trustd /trustd
ENTRYPOINT ["/trustd"]

# The osctl targets build the osctl binaries.

FROM base AS osctl-linux-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/cmd/osctl
ARG TARGETARCH
RUN GOOS=linux GOARCH=${TARGETARCH} go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-linux-${TARGETARCH}
RUN chmod +x /osctl-linux-${TARGETARCH}
FROM scratch AS osctl-linux
ARG TARGETARCH
COPY --from=osctl-linux-build /osctl-linux-${TARGETARCH} /osctl-linux-${TARGETARCH}

FROM base AS osctl-darwin-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/cmd/osctl
RUN GOOS=darwin GOARCH=${TARGETARCH} go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-darwin-${TARGETARCH}
RUN chmod +x /osctl-darwin-${TARGETARCH}
FROM scratch AS osctl-darwin
COPY --from=osctl-darwin-build /osctl-darwin-${TARGETARCH} /osctl-darwin-${TARGETARCH}

# The kernel target is the linux kernel.

FROM scratch AS kernel
ARG TARGETARCH
COPY --from=docker.io/andrewrynhard/kernel:fb78695 /boot/vmlinuz /vmlinuz-${TARGETARCH}

# The initramfs target provides the Talos initramfs image.

FROM scratch AS initramfs-base
COPY --from=docker.io/andrewrynhard/fhs:fb78695 / /
COPY --from=docker.io/andrewrynhard/musl:fb78695 / /
COPY --from=docker.io/andrewrynhard/ca-certificates:fb78695 / /
COPY --from=docker.io/andrewrynhard/dosfstools:fb78695 / /
COPY --from=docker.io/andrewrynhard/xfsprogs:fb78695 / /
COPY --from=docker.io/andrewrynhard/syslinux:fb78695 / /
FROM initramfs-base AS initramfs-build
COPY --from=init /init /init
FROM build AS initramfs-archive
COPY --from=initramfs-build / /initramfs
WORKDIR /initramfs
RUN set -o pipefail && find . 2>/dev/null | cpio -H newc -o | xz -v -C crc32 -0 -e -T 0 -z >/initramfs.xz
FROM scratch AS initramfs
ARG TARGETARCH
COPY --from=initramfs-archive /initramfs.xz /initramfs-${TARGETARCH}.xz

# The rootfs target provides the Talos rootfs image.

FROM initramfs-base AS rootfs-build
COPY --from=docker.io/andrewrynhard/libressl:fb78695 / /
COPY --from=docker.io/andrewrynhard/libseccomp:fb78695 / /
COPY --from=docker.io/andrewrynhard/iptables:fb78695 / /
COPY --from=docker.io/andrewrynhard/socat:fb78695 / /
COPY --from=docker.io/andrewrynhard/runc:fb78695 / /
COPY --from=docker.io/andrewrynhard/containerd:fb78695 / /
COPY --from=docker.io/andrewrynhard/crictl:fb78695 / /
COPY --from=docker.io/andrewrynhard/kubeadm:fb78695 / /
COPY --from=docker.io/andrewrynhard/cni:fb78695 / /
COPY --from=docker.io/andrewrynhard/images:fb78695 / /
COPY --from=docker.io/andrewrynhard/kernel:fb78695 /lib/modules /lib/modules
ARG TARGETOS
ARG TARGETARCH
COPY out/${TARGETOS}_${TARGETARCH}/images/*.tar /usr/images
FROM build AS rootfs-archive
COPY --from=rootfs-build / /rootfs
WORKDIR /rootfs
RUN tar -cvpzf /rootfs.tar.gz .
FROM scratch AS rootfs
ARG TARGETARCH
COPY --from=rootfs-archive /rootfs.tar.gz /rootfs-${TARGETARCH}.tar.gz

# The talos target generates a docker image that can be used to run Talos
# in containers.

FROM scratch AS talos
COPY --from=rootfs-build / /
COPY --from=init /init /init
ENTRYPOINT ["/init"]

# The installer target generates an image that can be used to install Talos to
# various environments.

FROM alpine:3.8 AS installer
ARG TARGETARCH
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
COPY --from=kernel /vmlinuz-${TARGETARCH} /usr/install/vmlinuz
COPY --from=initramfs-build /usr/lib/syslinux/ /usr/lib/syslinux
COPY --from=initramfs /initramfs-${TARGETARCH}.xz /usr/install/initramfs.xz
COPY --from=rootfs /rootfs-${TARGETARCH}.tar.gz /usr/install/rootfs.tar.gz
COPY --from=osctl-linux-build /osctl-linux-amd64 /bin/osctl
ARG TAG
ENV VERSION ${TAG}
ENTRYPOINT ["entrypoint.sh"]

# The test target performs tests on the source code.

FROM base AS test
COPY --from=rootfs-build / /rootfs
COPY hack/golang/test.sh /toolchain/bin

# The lint target performs linting on the source code.

FROM base AS lint
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /toolchain/bin v1.16.0
COPY hack/golang/golangci-lint.yaml .
RUN golangci-lint run --config golangci-lint.yaml
