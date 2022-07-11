# syntax = docker/dockerfile-upstream:1.2.0-labs

# Meta args applied to stage base names.

ARG TOOLS
ARG IMPORTVET
ARG PKGS
ARG EXTRAS
ARG INSTALLER_ARCH

# Resolve package images using ${PKGS} to be used later in COPY --from=.

FROM ghcr.io/siderolabs/fhs:${PKGS} AS pkg-fhs
FROM ghcr.io/siderolabs/ca-certificates:${PKGS} AS pkg-ca-certificates

FROM --platform=amd64 ghcr.io/siderolabs/cryptsetup:${PKGS} AS pkg-cryptsetup-amd64
FROM --platform=arm64 ghcr.io/siderolabs/cryptsetup:${PKGS} AS pkg-cryptsetup-arm64

FROM --platform=amd64 ghcr.io/siderolabs/containerd:${PKGS} AS pkg-containerd-amd64
FROM --platform=arm64 ghcr.io/siderolabs/containerd:${PKGS} AS pkg-containerd-arm64

FROM --platform=amd64 ghcr.io/siderolabs/dosfstools:${PKGS} AS pkg-dosfstools-amd64
FROM --platform=arm64 ghcr.io/siderolabs/dosfstools:${PKGS} AS pkg-dosfstools-arm64

FROM --platform=amd64 ghcr.io/siderolabs/eudev:${PKGS} AS pkg-eudev-amd64
FROM --platform=arm64 ghcr.io/siderolabs/eudev:${PKGS} AS pkg-eudev-arm64

FROM ghcr.io/siderolabs/grub:${PKGS} AS pkg-grub
FROM --platform=amd64 ghcr.io/siderolabs/grub:${PKGS} AS pkg-grub-amd64
FROM --platform=arm64 ghcr.io/siderolabs/grub:${PKGS} AS pkg-grub-arm64

FROM --platform=amd64 ghcr.io/siderolabs/iptables:${PKGS} AS pkg-iptables-amd64
FROM --platform=arm64 ghcr.io/siderolabs/iptables:${PKGS} AS pkg-iptables-arm64

FROM --platform=amd64 ghcr.io/siderolabs/libinih:${PKGS} AS pkg-libinih-amd64
FROM --platform=arm64 ghcr.io/siderolabs/libinih:${PKGS} AS pkg-libinih-arm64

FROM --platform=amd64 ghcr.io/siderolabs/libjson-c:${PKGS} AS pkg-libjson-c-amd64
FROM --platform=arm64 ghcr.io/siderolabs/libjson-c:${PKGS} AS pkg-libjson-c-arm64

FROM --platform=amd64 ghcr.io/siderolabs/libpopt:${PKGS} AS pkg-libpopt-amd64
FROM --platform=arm64 ghcr.io/siderolabs/libpopt:${PKGS} AS pkg-libpopt-arm64

FROM --platform=amd64 ghcr.io/siderolabs/liburcu:${PKGS} AS pkg-liburcu-amd64
FROM --platform=arm64 ghcr.io/siderolabs/liburcu:${PKGS} AS pkg-liburcu-arm64

FROM --platform=amd64 ghcr.io/siderolabs/openssl:${PKGS} AS pkg-openssl-amd64
FROM --platform=arm64 ghcr.io/siderolabs/openssl:${PKGS} AS pkg-openssl-arm64

FROM --platform=amd64 ghcr.io/siderolabs/libseccomp:${PKGS} AS pkg-libseccomp-amd64
FROM --platform=arm64 ghcr.io/siderolabs/libseccomp:${PKGS} AS pkg-libseccomp-arm64

# linux-firmware is not arch-specific
FROM --platform=amd64 ghcr.io/siderolabs/linux-firmware:${PKGS} AS pkg-linux-firmware

FROM --platform=amd64 ghcr.io/siderolabs/lvm2:${PKGS} AS pkg-lvm2-amd64
FROM --platform=arm64 ghcr.io/siderolabs/lvm2:${PKGS} AS pkg-lvm2-arm64

FROM --platform=amd64 ghcr.io/siderolabs/libaio:${PKGS} AS pkg-libaio-amd64
FROM --platform=arm64 ghcr.io/siderolabs/libaio:${PKGS} AS pkg-libaio-arm64

FROM --platform=amd64 ghcr.io/siderolabs/musl:${PKGS} AS pkg-musl-amd64
FROM --platform=arm64 ghcr.io/siderolabs/musl:${PKGS} AS pkg-musl-arm64

FROM --platform=amd64 ghcr.io/siderolabs/runc:${PKGS} AS pkg-runc-amd64
FROM --platform=arm64 ghcr.io/siderolabs/runc:${PKGS} AS pkg-runc-arm64

FROM --platform=amd64 ghcr.io/siderolabs/xfsprogs:${PKGS} AS pkg-xfsprogs-amd64
FROM --platform=arm64 ghcr.io/siderolabs/xfsprogs:${PKGS} AS pkg-xfsprogs-arm64

FROM --platform=amd64 ghcr.io/siderolabs/util-linux:${PKGS} AS pkg-util-linux-amd64
FROM --platform=arm64 ghcr.io/siderolabs/util-linux:${PKGS} AS pkg-util-linux-arm64

FROM --platform=amd64 ghcr.io/siderolabs/kmod:${PKGS} AS pkg-kmod-amd64
FROM --platform=arm64 ghcr.io/siderolabs/kmod:${PKGS} AS pkg-kmod-arm64

FROM ghcr.io/siderolabs/kernel:${PKGS} AS pkg-kernel
FROM --platform=amd64 ghcr.io/siderolabs/kernel:${PKGS} AS pkg-kernel-amd64
FROM --platform=arm64 ghcr.io/siderolabs/kernel:${PKGS} AS pkg-kernel-arm64

FROM --platform=arm64 ghcr.io/siderolabs/u-boot:${PKGS} AS pkg-u-boot-arm64
FROM --platform=arm64 ghcr.io/siderolabs/raspberrypi-firmware:${PKGS} AS pkg-raspberrypi-firmware-arm64

# Resolve package images using ${EXTRAS} to be used later in COPY --from=.

FROM ghcr.io/siderolabs/talosctl-cni-bundle-install:${EXTRAS} AS extras-talosctl-cni-bundle-install

# The tools target provides base toolchain for the build.

FROM $IMPORTVET as importvet

FROM --platform=${BUILDPLATFORM} $TOOLS AS tools
ENV PATH /toolchain/bin:/toolchain/go/bin
RUN ["/toolchain/bin/mkdir", "/bin", "/tmp"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/bin/bash", "/bin/sh"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/etc/ssl", "/etc/ssl"]
ARG GOLANGCILINT_VERSION
RUN curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/${GOLANGCILINT_VERSION}/install.sh | bash -s -- -b /toolchain/bin ${GOLANGCILINT_VERSION}
ARG GOIMPORTS_VERSION
RUN --mount=type=cache,target=/.cache go install golang.org/x/tools/cmd/goimports@${GOIMPORTS_VERSION} \
    && mv /go/bin/goimports /toolchain/go/bin/goimports
ARG GOFUMPT_VERSION
RUN --mount=type=cache,target=/.cache go install mvdan.cc/gofumpt@${GOFUMPT_VERSION} \
    && mv /go/bin/gofumpt /toolchain/go/bin/gofumpt
ARG DEEPCOPY_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/siderolabs/deep-copy@${DEEPCOPY_VERSION} \
    && mv /go/bin/deep-copy /toolchain/go/bin/deep-copy
ARG STRINGER_VERSION
RUN --mount=type=cache,target=/.cache go install golang.org/x/tools/cmd/stringer@${STRINGER_VERSION} \
    && mv /go/bin/stringer /toolchain/go/bin/stringer
ARG ENUMER_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/alvaroloes/enumer@${ENUMER_VERSION} \
    && mv /go/bin/enumer /toolchain/go/bin/enumer
ARG DEEPCOPY_GEN_VERSION
RUN --mount=type=cache,target=/.cache go install k8s.io/code-generator/cmd/deepcopy-gen@${DEEPCOPY_GEN_VERSION} \
    && mv /go/bin/deepcopy-gen /toolchain/go/bin/deepcopy-gen
ARG VTPROTOBUF_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@${VTPROTOBUF_VERSION} \
    && mv /go/bin/protoc-gen-go-vtproto /toolchain/go/bin/protoc-gen-go-vtproto
RUN curl -sfL https://github.com/uber/prototool/releases/download/v1.10.0/prototool-Linux-x86_64.tar.gz | tar -xz --strip-components=2 -C /toolchain/bin prototool/bin/prototool
COPY ./hack/docgen /go/src/github.com/talos-systems/talos-hack-docgen
RUN --mount=type=cache,target=/.cache cd /go/src/github.com/talos-systems/talos-hack-docgen \
    && go build -o docgen . \
    && mv docgen /toolchain/go/bin/
COPY --from=importvet /importvet /toolchain/go/bin/importvet

# The build target creates a container that will be used to build Talos source
# code.

FROM --platform=${BUILDPLATFORM} tools AS build
SHELL ["/toolchain/bin/bash", "-c"]
ENV PATH /toolchain/bin:/toolchain/go/bin
ENV GO111MODULE on
ENV GOPROXY https://proxy.golang.org
ARG CGO_ENABLED
ENV CGO_ENABLED ${CGO_ENABLED}
ENV GOCACHE /.cache/go-build
ENV GOMODCACHE /.cache/mod
ENV PROTOTOOL_CACHE_PATH /.cache/prototool
ARG SOURCE_DATE_EPOCH
ENV SOURCE_DATE_EPOCH ${SOURCE_DATE_EPOCH}
WORKDIR /src

# The build-go target creates a container to build Go code with Go modules downloaded and verified.

FROM build AS build-go
COPY ./go.mod ./go.sum ./
COPY ./pkg/machinery/go.mod ./pkg/machinery/go.sum ./pkg/machinery/
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache go mod download
WORKDIR /src
RUN --mount=type=cache,target=/.cache go mod download
RUN --mount=type=cache,target=/.cache go mod verify

# The generate target generates code from protobuf service definitions and machinery config.

# generate API descriptors
FROM build AS api-descriptors-build
WORKDIR /src/api
COPY api .
RUN --mount=type=cache,target=/.cache prototool format --overwrite --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include
RUN --mount=type=cache,target=/.cache prototool break descriptor-set --output-path=api.descriptors --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include

FROM --platform=${BUILDPLATFORM} scratch AS api-descriptors
COPY --from=api-descriptors-build /src/api/api.descriptors /api/api.descriptors

# format protobuf service definitions
FROM build AS proto-format-build
WORKDIR /src/api
COPY api .
RUN --mount=type=cache,target=/.cache prototool format --overwrite --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include

FROM --platform=${BUILDPLATFORM} scratch AS fmt-protobuf
COPY --from=proto-format-build /src/api/ /api/

# compile protobuf service definitions
FROM build AS generate-build
COPY --from=proto-format-build /src/api /api/
# Common needs to be at or near the top to satisfy the subsequent imports
COPY ./api/vendor/ /api/vendor/
COPY ./api/common/common.proto /api/common/common.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size common/common.proto
COPY ./api/security/security.proto /api/security/security.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size security/security.proto
COPY ./api/storage/storage.proto /api/storage/storage.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size storage/storage.proto
COPY ./api/machine/machine.proto /api/machine/machine.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size machine/machine.proto
COPY ./api/time/time.proto /api/time/time.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size time/time.proto
COPY ./api/cluster/cluster.proto /api/cluster/cluster.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size cluster/cluster.proto
COPY ./api/resource/resource.proto /api/resource/resource.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size resource/resource.proto
COPY ./api/resource/secrets/secrets.proto /api/resource/secrets/secrets.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size resource/secrets/secrets.proto
COPY ./api/inspect/inspect.proto /api/inspect/inspect.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size inspect/inspect.proto
# Goimports and gofumpt generated files to adjust import order
RUN goimports -w -local github.com/talos-systems/talos /api/
RUN gofumpt -w /api/

# run docgen for machinery config
FROM build-go AS go-generate
COPY ./pkg ./pkg
COPY ./hack/boilerplate.txt ./hack/boilerplate.txt
RUN --mount=type=cache,target=/.cache go generate ./pkg/...
RUN goimports -w -local github.com/talos-systems/talos ./pkg/
RUN gofumpt -w ./pkg/
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache go generate ./...
RUN goimports -w -local github.com/talos-systems/talos ./
RUN gofumpt -w ./

FROM build AS embed-generate
ARG NAME
ARG SHA
ARG USERNAME
ARG REGISTRY
ARG TAG
ARG ARTIFACTS
ARG PKGS
ARG EXTRAS
RUN mkdir -p pkg/machinery/gendata/data && \
    echo -n ${NAME} > pkg/machinery/gendata/data/name && \
    echo -n ${SHA} > pkg/machinery/gendata/data/sha && \
    echo -n ${USERNAME} > pkg/machinery/gendata/data/username && \
    echo -n ${REGISTRY} > pkg/machinery/gendata/data/registry && \
    echo -n ${EXTRAS} > pkg/machinery/gendata/data/extras && \
    echo -n ${PKGS} > pkg/machinery/gendata/data/pkgs && \
    echo -n ${TAG} > pkg/machinery/gendata/data/tag && \
    echo -n ${ARTIFACTS} > pkg/machinery/gendata/data/artifacts

FROM scratch AS embed
COPY --from=embed-generate /src/pkg/machinery/gendata/data /pkg/machinery/gendata/data

FROM embed-generate AS embed-abbrev-generate
ARG ABBREV_TAG
RUN echo -n "undefined" > pkg/machinery/gendata/data/sha && \
    echo -n ${ABBREV_TAG} > pkg/machinery/gendata/data/tag

FROM scratch AS embed-abbrev
COPY --from=embed-abbrev-generate /src/pkg/machinery/gendata/data /pkg/machinery/gendata/data

FROM --platform=${BUILDPLATFORM} scratch AS generate
COPY --from=proto-format-build /src/api /api/
COPY --from=generate-build /api/common/*.pb.go /pkg/machinery/api/common/
COPY --from=generate-build /api/security/*.pb.go /pkg/machinery/api/security/
COPY --from=generate-build /api/machine/*.pb.go /pkg/machinery/api/machine/
COPY --from=generate-build /api/time/*.pb.go /pkg/machinery/api/time/
COPY --from=generate-build /api/cluster/*.pb.go /pkg/machinery/api/cluster/
COPY --from=generate-build /api/storage/*.pb.go /pkg/machinery/api/storage/
COPY --from=generate-build /api/resource/*.pb.go /pkg/machinery/api/resource/
COPY --from=generate-build /api/resource/secrets/*.pb.go /pkg/machinery/api/resource/secrets/
COPY --from=generate-build /api/inspect/*.pb.go /pkg/machinery/api/inspect/
COPY --from=go-generate /src/pkg/machinery/resources/kubespan/ /pkg/machinery/resources/kubespan/
COPY --from=go-generate /src/pkg/machinery/resources/network/ /pkg/machinery/resources/network/
COPY --from=go-generate /src/pkg/machinery/config/types/v1alpha1/ /pkg/machinery/config/types/v1alpha1/
COPY --from=go-generate /src/pkg/machinery/nethelpers/ /pkg/machinery/nethelpers/
COPY --from=go-generate /src/pkg/machinery/extensions/ /pkg/machinery/extensions/
COPY --from=embed-abbrev / /

# The base target provides a container that can be used to build all Talos
# assets.

FROM build-go AS base
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY --from=generate /pkg/machinery/ ./pkg/machinery/
COPY --from=embed / ./
RUN --mount=type=cache,target=/.cache go list all >/dev/null
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache go mod download
RUN --mount=type=cache,target=/.cache go list all >/dev/null
WORKDIR /src

# The init target builds the init binary.

FROM base AS init-build-amd64
WORKDIR /src/internal/app/init
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /init
RUN chmod +x /init

FROM base AS init-build-arm64
WORKDIR /src/internal/app/init
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=arm64 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /init
RUN chmod +x /init

FROM init-build-${TARGETARCH} AS init-build

FROM scratch AS init
COPY --from=init-build /init /init

# The machined target builds the machined binary.

FROM base AS machined-build-amd64
WORKDIR /src/internal/app/machined
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /machined
RUN chmod +x /machined

FROM base AS machined-build-arm64
WORKDIR /src/internal/app/machined
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=arm64 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /machined
RUN chmod +x /machined

FROM machined-build-${TARGETARCH} AS machined-build

FROM scratch AS machined
COPY --from=machined-build /machined /machined

# The talosctl targets build the talosctl binaries.

FROM base AS talosctl-linux-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-amd64
RUN chmod +x /talosctl-linux-amd64

FROM base AS talosctl-linux-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=arm64 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-arm64
RUN chmod +x /talosctl-linux-arm64

FROM base AS talosctl-linux-armv7-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=arm GOARM=7 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-armv7
RUN chmod +x /talosctl-linux-armv7

FROM scratch AS talosctl-linux
COPY --from=talosctl-linux-amd64-build /talosctl-linux-amd64 /talosctl-linux-amd64
COPY --from=talosctl-linux-arm64-build /talosctl-linux-arm64 /talosctl-linux-arm64
COPY --from=talosctl-linux-armv7-build /talosctl-linux-armv7 /talosctl-linux-armv7

FROM scratch as talosctl
ARG TARGETARCH
COPY --from=talosctl-linux /talosctl-linux-${TARGETARCH} /talosctl
ARG TAG
ENV VERSION ${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
LABEL org.opencontainers.image.source https://github.com/siderolabs/talos
ENTRYPOINT ["/talosctl"]

FROM base AS talosctl-darwin-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=darwin GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talosctl-darwin-amd64
RUN chmod +x /talosctl-darwin-amd64

FROM base AS talosctl-darwin-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=darwin GOARCH=arm64 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talosctl-darwin-arm64
RUN chmod +x /talosctl-darwin-arm64

FROM scratch AS talosctl-darwin
COPY --from=talosctl-darwin-amd64-build /talosctl-darwin-amd64 /talosctl-darwin-amd64
COPY --from=talosctl-darwin-arm64-build /talosctl-darwin-arm64 /talosctl-darwin-arm64

FROM base AS talosctl-windows-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=windows GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /talosctl-windows-amd64.exe

FROM scratch AS talosctl-windows
COPY --from=talosctl-windows-amd64-build /talosctl-windows-amd64.exe /talosctl-windows-amd64.exe

# The kernel target is the linux kernel.

FROM scratch AS kernel
ARG TARGETARCH
COPY --from=pkg-kernel /boot/vmlinuz /vmlinuz-${TARGETARCH}

# The rootfs target provides the Talos rootfs.

FROM build AS rootfs-base-amd64
COPY --from=pkg-fhs / /rootfs
COPY --from=pkg-ca-certificates / /rootfs
COPY --from=pkg-cryptsetup-amd64 / /rootfs
COPY --from=pkg-containerd-amd64 / /rootfs
COPY --from=pkg-dosfstools-amd64 / /rootfs
COPY --from=pkg-eudev-amd64 / /rootfs
COPY --from=pkg-iptables-amd64 / /rootfs
COPY --from=pkg-libinih-amd64 / /rootfs
COPY --from=pkg-libjson-c-amd64 / /rootfs
COPY --from=pkg-libpopt-amd64 / /rootfs
COPY --from=pkg-liburcu-amd64 / /rootfs
COPY --from=pkg-openssl-amd64 / /rootfs
COPY --from=pkg-libseccomp-amd64 / /rootfs
COPY --from=pkg-lvm2-amd64 / /rootfs
COPY --from=pkg-libaio-amd64 / /rootfs
COPY --from=pkg-musl-amd64 / /rootfs
COPY --from=pkg-runc-amd64 / /rootfs
COPY --from=pkg-xfsprogs-amd64 / /rootfs
COPY --from=pkg-util-linux-amd64 /lib/libblkid.* /rootfs/lib/
COPY --from=pkg-util-linux-amd64 /lib/libuuid.* /rootfs/lib/
COPY --from=pkg-util-linux-amd64 /lib/libmount.* /rootfs/lib/
COPY --from=pkg-kmod-amd64 /usr/lib/libkmod.* /rootfs/lib/
COPY --from=pkg-kernel-amd64 /lib/modules /rootfs/lib/modules
COPY --from=machined-build-amd64 /machined /rootfs/sbin/init
# the orderly_poweroff call by the kernel will call '/sbin/poweroff'
RUN ln /rootfs/sbin/init /rootfs/sbin/poweroff
RUN chmod +x /rootfs/sbin/poweroff
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
COPY ./hack/cleanup.sh /toolchain/bin/cleanup.sh
RUN cleanup.sh /rootfs
RUN mkdir -pv /rootfs/{boot,etc/cri/conf.d/hosts,lib/firmware,usr/local/share,usr/share/zoneinfo/Etc,mnt,system,opt}
COPY --chmod=0644 hack/zoneinfo/Etc/UTC /rootfs/usr/share/zoneinfo/Etc/UTC
RUN ln -s /usr/share/zoneinfo/Etc/UTC /rootfs/etc/localtime
RUN mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni/net.d,usr/libexec/kubernetes}
RUN mkdir -pv /rootfs/opt/{containerd/bin,containerd/lib}
COPY --chmod=0644 hack/containerd.toml /rootfs/etc/containerd/config.toml
COPY --chmod=0644 hack/cri-containerd.toml /rootfs/etc/cri/containerd.toml
COPY --chmod=0644 hack/cri-plugin.part /rootfs/etc/cri/conf.d/00-base.part
RUN touch /rootfs/etc/{extensions.yaml,resolv.conf,hosts,os-release,machine-id,cri/conf.d/cri.toml,cri/conf.d/01-registries.part}
RUN ln -s ca-certificates /rootfs/etc/ssl/certs/ca-certificates.crt
RUN ln -s /etc/ssl /rootfs/etc/pki
RUN ln -s /etc/ssl /rootfs/usr/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/etc/ca-certificates

FROM build AS rootfs-base-arm64
COPY --from=pkg-fhs / /rootfs
COPY --from=pkg-ca-certificates / /rootfs
COPY --from=pkg-cryptsetup-arm64 / /rootfs
COPY --from=pkg-containerd-arm64 / /rootfs
COPY --from=pkg-dosfstools-arm64 / /rootfs
COPY --from=pkg-eudev-arm64 / /rootfs
COPY --from=pkg-iptables-arm64 / /rootfs
COPY --from=pkg-libinih-arm64 / /rootfs
COPY --from=pkg-libjson-c-arm64 / /rootfs
COPY --from=pkg-libpopt-arm64 / /rootfs
COPY --from=pkg-liburcu-arm64 / /rootfs
COPY --from=pkg-openssl-arm64 / /rootfs
COPY --from=pkg-libseccomp-arm64 / /rootfs
COPY --from=pkg-lvm2-arm64 / /rootfs
COPY --from=pkg-libaio-arm64 / /rootfs
COPY --from=pkg-musl-arm64 / /rootfs
COPY --from=pkg-runc-arm64 / /rootfs
COPY --from=pkg-xfsprogs-arm64 / /rootfs
COPY --from=pkg-util-linux-arm64 /lib/libblkid.* /rootfs/lib/
COPY --from=pkg-util-linux-arm64 /lib/libuuid.* /rootfs/lib/
COPY --from=pkg-util-linux-arm64 /lib/libmount.* /rootfs/lib/
COPY --from=pkg-kmod-arm64 /usr/lib/libkmod.* /rootfs/lib/
COPY --from=pkg-kernel-arm64 /lib/modules /rootfs/lib/modules
COPY --from=machined-build-arm64 /machined /rootfs/sbin/init
# the orderly_poweroff call by the kernel will call '/sbin/poweroff'
RUN ln /rootfs/sbin/init /rootfs/sbin/poweroff
RUN chmod +x /rootfs/sbin/poweroff
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
COPY ./hack/cleanup.sh /toolchain/bin/cleanup.sh
RUN cleanup.sh /rootfs
RUN mkdir -pv /rootfs/{boot,etc/cri/conf.d/hosts,lib/firmware,usr/local/share,usr/share/zoneinfo/Etc,mnt,system,opt}
COPY --chmod=0644 hack/zoneinfo/Etc/UTC /rootfs/usr/share/zoneinfo/Etc/UTC
RUN ln -s /usr/share/zoneinfo/Etc/UTC /rootfs/etc/localtime
RUN mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni/net.d,usr/libexec/kubernetes}
RUN mkdir -pv /rootfs/opt/{containerd/bin,containerd/lib}
COPY --chmod=0644 hack/containerd.toml /rootfs/etc/containerd/config.toml
COPY --chmod=0644 hack/cri-containerd.toml /rootfs/etc/cri/containerd.toml
COPY --chmod=0644 hack/cri-plugin.part /rootfs/etc/cri/conf.d/00-base.part
RUN touch /rootfs/etc/{extensions.yaml,resolv.conf,hosts,os-release,machine-id,cri/conf.d/cri.toml,cri/conf.d/01-registries.part}
RUN ln -s /etc/ssl /rootfs/etc/pki
RUN ln -s ca-certificates /rootfs/etc/ssl/certs/ca-certificates.crt
RUN ln -s /etc/ssl /rootfs/usr/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/etc/ca-certificates

FROM rootfs-base-${TARGETARCH} AS rootfs-base

FROM rootfs-base-arm64 AS rootfs-squashfs-arm64
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
RUN mksquashfs /rootfs /rootfs.sqsh -all-root -noappend -comp xz -Xdict-size 100% -no-progress

FROM rootfs-base-amd64 AS rootfs-squashfs-amd64
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
RUN mksquashfs /rootfs /rootfs.sqsh -all-root -noappend -comp xz -Xdict-size 100% -no-progress

FROM scratch AS squashfs-arm64
COPY --from=rootfs-squashfs-arm64 /rootfs.sqsh /

FROM scratch AS squashfs-amd64
COPY --from=rootfs-squashfs-amd64 /rootfs.sqsh /

FROM scratch AS rootfs
COPY --from=rootfs-base /rootfs /

# The initramfs target provides the Talos initramfs image.

FROM build AS initramfs-archive-arm64
WORKDIR /initramfs
COPY --from=squashfs-arm64 /rootfs.sqsh .
COPY --from=init-build-arm64 /init .
# copying over firmware binary blobs to initramfs
COPY --from=pkg-linux-firmware /lib/firmware/rtl_nic ./lib/firmware/rtl_nic
COPY --from=pkg-linux-firmware /lib/firmware/nvidia/tegra210 ./lib/firmware/nvidia/tegra210
RUN find . -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
RUN set -o pipefail \
    && find . 2>/dev/null \
    | LC_ALL=c sort \
    | cpio --reproducible -H newc -o \
    | xz -v -C crc32 -0 -e -T 0 -z \
    > /initramfs.xz

FROM build AS initramfs-archive-amd64
WORKDIR /initramfs
COPY --from=squashfs-amd64 /rootfs.sqsh .
COPY --from=init-build-amd64 /init .
# copying over firmware binary blobs to initramfs
COPY --from=pkg-linux-firmware /lib/firmware/bnx2 ./lib/firmware/bnx2
COPY --from=pkg-linux-firmware /lib/firmware/bnx2x ./lib/firmware/bnx2x
RUN find . -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
RUN set -o pipefail \
    && find . 2>/dev/null \
    | LC_ALL=c sort \
    | cpio --reproducible -H newc -o \
    | xz -v -C crc32 -0 -e -T 0 -z \
    > /initramfs.xz

FROM initramfs-archive-${TARGETARCH} AS initramfs-archive

FROM scratch AS initramfs
ARG TARGETARCH
COPY --from=initramfs-archive /initramfs.xz /initramfs-${TARGETARCH}.xz

# The talos target generates a docker image that can be used to run Talos
# in containers.

FROM scratch AS talos
COPY --from=rootfs / /
LABEL org.opencontainers.image.source https://github.com/siderolabs/talos
ENTRYPOINT ["/sbin/init"]

# The installer target generates an image that can be used to install Talos to
# various environments.

FROM base AS installer-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
WORKDIR /src/cmd/installer
ARG TARGETARCH
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=${TARGETARCH} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /installer
RUN chmod +x /installer

FROM alpine:3.16.0 AS unicode-pf2
RUN apk add --no-cache --update --no-scripts grub

FROM scratch AS install-artifacts-amd64
COPY --from=pkg-grub-amd64 /usr/lib/grub /usr/lib/grub
COPY --from=pkg-kernel-amd64 /boot/vmlinuz /usr/install/amd64/vmlinuz
COPY --from=pkg-kernel-amd64 /dtb /usr/install/amd64/dtb
COPY --from=initramfs-archive-amd64 /initramfs.xz /usr/install/amd64/initramfs.xz

FROM scratch AS install-artifacts-arm64
COPY --from=pkg-grub-arm64 /usr/lib/grub /usr/lib/grub
COPY --from=pkg-kernel-arm64 /boot/vmlinuz /usr/install/arm64/vmlinuz
COPY --from=pkg-kernel-arm64 /dtb /usr/install/arm64/dtb
COPY --from=initramfs-archive-arm64 /initramfs.xz /usr/install/arm64/initramfs.xz
COPY --from=pkg-u-boot-arm64 / /usr/install/arm64/u-boot
COPY --from=pkg-raspberrypi-firmware-arm64 / /usr/install/arm64/raspberrypi-firmware

FROM scratch AS install-artifacts-all
COPY --from=install-artifacts-amd64 / /
COPY --from=install-artifacts-arm64 / /

FROM install-artifacts-${TARGETARCH} AS install-artifacts-targetarch

FROM install-artifacts-${INSTALLER_ARCH} AS install-artifacts
COPY --from=pkg-grub / /
COPY --from=unicode-pf2 /usr/share/grub/unicode.pf2 /usr/share/grub/unicode.pf2

FROM alpine:3.16.0 AS installer
RUN apk add --no-cache --update --no-scripts \
    bash \
    cpio \
    efibootmgr \
    mtools \
    qemu-img \
    squashfs-tools \
    util-linux \
    xfsprogs \
    xorriso \
    xz
ARG TARGETARCH
ENV TARGETARCH ${TARGETARCH}
COPY --from=install-artifacts / /
COPY --from=installer-build /installer /bin/installer
RUN ln -s /bin/installer /bin/talosctl
ARG TAG
ENV VERSION ${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
LABEL org.opencontainers.image.source https://github.com/siderolabs/talos
ENTRYPOINT ["/bin/installer"]
ONBUILD RUN apk add --no-cache --update \
    cpio \
    squashfs-tools \
    xz
ONBUILD WORKDIR /initramfs
ONBUILD ARG RM
ONBUILD RUN xz -d /usr/install/${TARGETARCH}/initramfs.xz \
    && cpio -idvm < /usr/install/${TARGETARCH}/initramfs \
    && unsquashfs -f -d /rootfs rootfs.sqsh \
    && for f in ${RM}; do rm -rfv /rootfs$f; done \
    && rm /usr/install/${TARGETARCH}/initramfs \
    && rm rootfs.sqsh
ONBUILD COPY --from=customization / /rootfs
ONBUILD RUN find /rootfs \
    && mksquashfs /rootfs rootfs.sqsh -all-root -noappend -comp xz -Xdict-size 100% -no-progress \
    && set -o pipefail && find . 2>/dev/null | cpio -H newc -o | xz -v -C crc32 -0 -e -T 0 -z >/usr/install/${TARGETARCH}/initramfs.xz \
    && rm -rf /rootfs \
    && rm -rf /initramfs
ONBUILD WORKDIR /

FROM installer AS imager

# The test target performs tests on the source code.

FROM base AS unit-tests-runner
RUN unlink /etc/ssl
COPY --from=rootfs / /
ARG TESTPKGS
ENV PLATFORM container
ARG GO_LDFLAGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache go test -v \
    -ldflags "${GO_LDFLAGS}" \
    -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 -p 4 ${TESTPKGS}
FROM scratch AS unit-tests
COPY --from=unit-tests-runner /src/coverage.txt /coverage.txt

# The unit-tests-race target performs tests with race detector.

FROM base AS unit-tests-race
RUN unlink /etc/ssl
COPY --from=rootfs / /
ARG TESTPKGS
ENV PLATFORM container
ENV CGO_ENABLED 1
ARG GO_LDFLAGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache go test -v \
    -ldflags "${GO_LDFLAGS}" \
    -race -count 1 -p 4 ${TESTPKGS}

# The integration-test targets builds integration test binary.

FROM base AS integration-test-linux-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-linux
COPY --from=integration-test-linux-build /src/integration.test /integration-test-linux-amd64

FROM base AS integration-test-darwin-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=darwin GOARCH=amd64 GOAMD64=${GOAMD64} go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-darwin
COPY --from=integration-test-darwin-build /src/integration.test /integration-test-darwin-amd64

# The integration-test-provision target builds integration test binary with provisioning tests.

FROM base AS integration-test-provision-linux-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_provision \
    ./internal/integration

FROM scratch AS integration-test-provision-linux
COPY --from=integration-test-provision-linux-build /src/integration.test /integration-test-provision-linux-amd64

# The lint target performs linting on the source code.

FROM base AS lint-go
COPY .golangci.yml .
ENV GOGC 50
ENV GOLANGCI_LINT_CACHE /.cache/lint
RUN --mount=type=cache,target=/.cache golangci-lint run --config .golangci.yml
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache golangci-lint run --config ../../.golangci.yml
WORKDIR /src
RUN --mount=type=cache,target=/.cache importvet github.com/talos-systems/talos/...

# The protolint target performs linting on protobuf files.

FROM base AS lint-protobuf
WORKDIR /src/api
COPY api .
RUN --mount=type=cache,target=/.cache prototool lint --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include
RUN --mount=type=cache,target=/.cache prototool break check --descriptor-set-path=api.descriptors --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include

# The markdownlint target performs linting on Markdown files.

FROM node:18.5.0-alpine AS lint-markdown
RUN apk add --no-cache findutils
RUN npm i -g markdownlint-cli@0.23.2
RUN npm i -g textlint@11.7.6
RUN npm i -g textlint-filter-rule-comments@1.2.2
RUN npm i -g textlint-rule-one-sentence-per-line@1.0.2
WORKDIR /src
COPY . .
RUN markdownlint \
    --ignore '**/LICENCE.md' \
    --ignore '**/CHANGELOG.md' \
    --ignore '**/CODE_OF_CONDUCT.md' \
    --ignore '**/node_modules/**' \
    --ignore '**/hack/chglog/**' \
    --ignore 'website/content/*/reference/*' \
    --ignore 'website/themes/**' \
    .
RUN find . \
    -name '*.md' \
    -not -path './LICENCE.md' \
    -not -path './CHANGELOG.md' \
    -not -path './CODE_OF_CONDUCT.md' \
    -not -path '*/node_modules/*' \
    -not -path './hack/chglog/**' \
    -not -path './website/content/*/reference/*' \
    -not -path './website/themes/**' \
    -print0 \
    | xargs -0 textlint

# The docs target generates documentation.

FROM base AS docs-build
WORKDIR /src
COPY --from=talosctl-linux /talosctl-linux-amd64 /bin/talosctl
RUN env HOME=/home/user TAG=latest /bin/talosctl docs --config /tmp \
    && env HOME=/home/user TAG=latest /bin/talosctl docs --cli /tmp

FROM pseudomuto/protoc-gen-doc as proto-docs-build
COPY --from=generate-build /api /protos
COPY ./hack/protoc-gen-doc/markdown.tmpl /tmp/markdown.tmpl
RUN protoc \
    -I/protos \
    -I/protos/common \
    -I/protos/inspect \
    -I/protos/machine \
    -I/protos/resource \
    -I/protos/security \
    -I/protos/storage \
    -I/protos/time \
    -I/protos/vendor \
    --doc_opt=/tmp/markdown.tmpl,api.md \
    --doc_out=/tmp \
    /protos/common/*.proto \
    /protos/inspect/*.proto \
    /protos/machine/*.proto \
    /protos/resource/*.proto \
    /protos/security/*.proto \
    /protos/storage/*.proto \
    /protos/time/*.proto

FROM scratch AS docs
COPY --from=docs-build /tmp/configuration.md /website/content/v1.2/reference/
COPY --from=docs-build /tmp/cli.md /website/content/v1.2/reference/
COPY --from=proto-docs-build /tmp/api.md /website/content/v1.2/reference/

# The talosctl-cni-bundle builds the CNI bundle for talosctl.

FROM scratch AS talosctl-cni-bundle
ARG TARGETARCH
COPY --from=extras-talosctl-cni-bundle-install /opt/cni/bin/ /talosctl-cni-bundle-${TARGETARCH}/

# The go-mod-outdated target lists all outdated modules.

FROM base AS go-mod-outdated
RUN --mount=type=cache,target=/.cache go install github.com/psampaz/go-mod-outdated@latest \
    && mv /go/bin/go-mod-outdated /toolchain/go/bin/go-mod-outdated
COPY ./hack/cloud-image-uploader ./hack/cloud-image-uploader
# fail always to get the output back
RUN --mount=type=cache,target=/.cache cd pkg/machinery && \
    echo -e "\n>>>> pkg/machinery:" && \
    (go list -u -m -json all | go-mod-outdated -update -direct) && \
    cd ../.. && \
    echo -e "\n>>>> .:" && \
    (go list -u -m -json all | go-mod-outdated -update -direct) && \
    cd hack/cloud-image-uploader && \
    echo -e "\n>>>> hack/cloud-image-uploader:" && \
    (go list -u -m -json all | go-mod-outdated -update -direct) && \
    exit 1
