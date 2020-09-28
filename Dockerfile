# syntax = docker/dockerfile-upstream:1.1.7-experimental

# Meta args applied to stage base names.

ARG TOOLS
ARG IMPORTVET
ARG PKGS

# Resolve package images using ${PKGS} to be used later in COPY --from=.

FROM ghcr.io/talos-systems/fhs:${PKGS} AS pkg-fhs
FROM ghcr.io/talos-systems/ca-certificates:${PKGS} AS pkg-ca-certificates
FROM ghcr.io/talos-systems/containerd:${PKGS} AS pkg-containerd
FROM ghcr.io/talos-systems/dosfstools:${PKGS} AS pkg-dosfstools
FROM ghcr.io/talos-systems/eudev:${PKGS} AS pkg-eudev
FROM ghcr.io/talos-systems/grub:${PKGS} AS pkg-grub
FROM ghcr.io/talos-systems/iptables:${PKGS} AS pkg-iptables
FROM ghcr.io/talos-systems/libressl:${PKGS} AS pkg-libressl
FROM ghcr.io/talos-systems/libseccomp:${PKGS} AS pkg-libseccomp
FROM ghcr.io/talos-systems/linux-firmware:${PKGS} AS pkg-linux-firmware
FROM ghcr.io/talos-systems/linux-firmware:${PKGS} AS pkg-linux-firmware
FROM ghcr.io/talos-systems/lvm2:${PKGS} AS pkg-lvm2
FROM ghcr.io/talos-systems/libaio:${PKGS} AS pkg-libaio
FROM ghcr.io/talos-systems/musl:${PKGS} AS pkg-musl
FROM ghcr.io/talos-systems/open-iscsi:${PKGS} AS pkg-open-iscsi
FROM ghcr.io/talos-systems/open-isns:${PKGS} AS pkg-open-isns
FROM ghcr.io/talos-systems/runc:${PKGS} AS pkg-runc
FROM ghcr.io/talos-systems/socat:${PKGS} AS pkg-socat
FROM ghcr.io/talos-systems/xfsprogs:${PKGS} AS pkg-xfsprogs
FROM ghcr.io/talos-systems/util-linux:${PKGS} AS pkg-util-linux
FROM ghcr.io/talos-systems/util-linux:${PKGS} AS pkg-util-linux
FROM ghcr.io/talos-systems/util-linux:${PKGS} AS pkg-util-linux
FROM ghcr.io/talos-systems/kmod:${PKGS} AS pkg-kmod
FROM ghcr.io/talos-systems/kernel:${PKGS} AS pkg-kernel

# The tools target provides base toolchain for the build.

FROM $IMPORTVET as importvet

FROM $TOOLS AS tools
ENV PATH /toolchain/bin:/toolchain/go/bin
RUN ["/toolchain/bin/mkdir", "/bin", "/tmp"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/bin/bash", "/bin/sh"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/etc/ssl", "/etc/ssl"]
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /toolchain/bin v1.28.3
ARG GOFUMPT_VERSION
RUN cd $(mktemp -d) \
    && go mod init tmp \
    && go get mvdan.cc/gofumpt/gofumports@${GOFUMPT_VERSION} \
    && mv /go/bin/gofumports /toolchain/go/bin/gofumports
RUN curl -sfL https://github.com/uber/prototool/releases/download/v1.8.0/prototool-Linux-x86_64.tar.gz | tar -xz --strip-components=2 -C /toolchain/bin prototool/bin/prototool
COPY ./hack/docgen /go/src/github.com/talos-systems/docgen
RUN cd /go/src/github.com/talos-systems/docgen \
    && go build . \
    && mv docgen /toolchain/go/bin/
COPY --from=importvet /importvet /toolchain/go/bin/importvet

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
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api common/common.proto
COPY ./api/health/health.proto /api/health/health.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api health/health.proto
COPY ./api/security/security.proto /api/security/security.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api security/security.proto
COPY ./api/machine/machine.proto /api/machine/machine.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api machine/machine.proto
COPY ./api/time/time.proto /api/time/time.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api time/time.proto
COPY ./api/network/network.proto /api/network/network.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api network/network.proto
COPY ./api/os/os.proto /api/os/os.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api os/os.proto
COPY ./api/cluster/cluster.proto /api/cluster/cluster.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api cluster/cluster.proto
# Gofumports generated files to adjust import order
RUN gofumports -w -local github.com/talos-systems/talos /api/

FROM scratch AS generate
COPY --from=generate-build /api/common/common.pb.go /pkg/machinery/api/common/
COPY --from=generate-build /api/health/health.pb.go /pkg/machinery/api/health/
COPY --from=generate-build /api/os/os.pb.go /pkg/machinery/api/os/
COPY --from=generate-build /api/security/security.pb.go /pkg/machinery/api/security/
COPY --from=generate-build /api/machine/machine.pb.go /pkg/machinery/api/machine/
COPY --from=generate-build /api/time/time.pb.go /pkg/machinery/api/time/
COPY --from=generate-build /api/network/network.pb.go /pkg/machinery/api/network/
COPY --from=generate-build /api/cluster/cluster.pb.go /pkg/machinery/api/cluster/

# The base target provides a container that can be used to build all Talos
# assets.

FROM build AS base
COPY ./go.mod ./go.sum ./
COPY ./pkg/machinery/go.mod ./pkg/machinery/go.sum ./pkg/machinery/
RUN go mod download
RUN go mod verify
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY --from=generate /pkg/machinery/api ./pkg/machinery/api
RUN go list -mod=readonly all >/dev/null
RUN ! go mod tidy -v 2>&1 | grep .

# The init target builds the init binary.

FROM base AS init-build
ARG SHA
ARG TAG
ARG PKGS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/init
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS}" -o /init
RUN chmod +x /init

FROM scratch AS init
COPY --from=init-build /init /init

# The machined target builds the machined image.

FROM base AS machined-build
ARG SHA
ARG TAG
ARG PKGS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
WORKDIR /src/internal/app/machined
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${IMAGES_PKGS}.Registry=${REGISTRY}" -o /machined
RUN chmod +x /machined

FROM scratch AS machined
COPY --from=machined-build /machined /machined

# The timed target builds the timed image.

FROM base AS timed-build
ARG SHA
ARG TAG
ARG PKGS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/timed
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS}" -o /timed
RUN chmod +x /timed

FROM base AS timed-image
ARG TAG
ARG USERNAME
COPY --from=timed-build /timed /scratch/timed
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./timed /timed\nENTRYPOINT [\"/timed\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/timed:${TAG} --output type=docker,dest=/timed.tar --no-console  .

# The apid target builds the api image.

FROM base AS apid-build
ARG SHA
ARG TAG
ARG PKGS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/apid
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS}" -o /apid
RUN chmod +x /apid

FROM base AS apid-image
ARG TAG
ARG USERNAME
COPY --from=apid-build /apid /scratch/apid
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./apid /apid\nENTRYPOINT [\"/apid\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/apid:${TAG} --output type=docker,dest=/apid.tar --no-console  .

# The trustd target builds the trustd image.

FROM base AS trustd-build
ARG SHA
ARG TAG
ARG PKGS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/trustd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS}" -o /trustd
RUN chmod +x /trustd

FROM base AS trustd-image
ARG TAG
ARG USERNAME
COPY --from=trustd-build /trustd /scratch/trustd
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./trustd /trustd\nENTRYPOINT [\"/trustd\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/trustd:${TAG} --output type=docker,dest=/trustd.tar --no-console  .

# The networkd target builds the networkd image.

FROM base AS networkd-build
ARG SHA
ARG TAG
ARG PKGS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/networkd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS}" -o /networkd
RUN chmod +x /networkd

FROM base AS networkd-image
ARG TAG
ARG USERNAME
COPY --from=networkd-build /networkd /scratch/networkd
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./networkd /networkd\nENTRYPOINT [\"/networkd\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/networkd:${TAG} --output type=docker,dest=/networkd.tar --no-console  .

# The routerd target builds the routerd image.

FROM base AS routerd-build
ARG SHA
ARG TAG
ARG PKGS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/routerd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS}" -o /routerd
RUN chmod +x /routerd

FROM base AS routerd-image
ARG TAG
ARG USERNAME
COPY --from=routerd-build /routerd /scratch/routerd
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./routerd /routerd\nENTRYPOINT [\"/routerd\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/routerd:${TAG} --output type=docker,dest=/routerd.tar --no-console  .


# The bootkube target builds the bootkube image.

FROM base AS bootkube-build
ARG SHA
ARG TAG
ARG PKGS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/bootkube
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS}" -o /bootkube
RUN chmod +x /bootkube

FROM base AS bootkube-image
ARG TAG
ARG USERNAME
COPY --from=bootkube-build /bootkube /scratch/bootkube
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./bootkube /bootkube\nENTRYPOINT [\"/bootkube\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/bootkube:${TAG} --output type=docker,dest=/bootkube.tar --no-console  .


# The talosctl targets build the talosctl binaries.

FROM base AS talosctl-linux-amd64-build
ARG SHA
ARG TAG
ARG PKGS
ARG ARTIFACTS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${IMAGES_PKGS}.Registry=${REGISTRY} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-linux-amd64
RUN chmod +x /talosctl-linux-amd64

FROM base AS talosctl-linux-arm64-build
ARG SHA
ARG TAG
ARG PKGS
ARG ARTIFACTS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${IMAGES_PKGS}.Registry=${REGISTRY} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-linux-arm64
RUN chmod +x /talosctl-linux-arm64

FROM base AS talosctl-linux-armv7-build
ARG SHA
ARG TAG
ARG PKGS
ARG ARTIFACTS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=arm GOARM=7  go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${IMAGES_PKGS}.Registry=${REGISTRY} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-linux-armv7
RUN chmod +x /talosctl-linux-armv7

FROM scratch AS talosctl-linux
COPY --from=talosctl-linux-amd64-build /talosctl-linux-amd64 /talosctl-linux-amd64
COPY --from=talosctl-linux-arm64-build /talosctl-linux-arm64 /talosctl-linux-arm64
COPY --from=talosctl-linux-armv7-build /talosctl-linux-armv7 /talosctl-linux-armv7

FROM base AS talosctl-darwin-build
ARG SHA
ARG TAG
ARG PKGS
ARG ARTIFACTS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${IMAGES_PKGS}.Registry=${REGISTRY} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-darwin-amd64
RUN chmod +x /talosctl-darwin-amd64

FROM scratch AS talosctl-darwin
COPY --from=talosctl-darwin-build /talosctl-darwin-amd64 /talosctl-darwin-amd64

# The kernel target is the linux kernel.

FROM scratch AS kernel
ARG TARGETARCH
COPY --from=pkg-kernel /boot/vmlinuz /vmlinuz-${TARGETARCH}

# The rootfs target provides the Talos rootfs.

FROM build AS rootfs-base
COPY --from=pkg-fhs / /rootfs
COPY --from=pkg-ca-certificates / /rootfs
COPY --from=pkg-containerd / /rootfs
COPY --from=pkg-dosfstools / /rootfs
COPY --from=pkg-eudev / /rootfs
COPY --from=pkg-iptables / /rootfs
COPY --from=pkg-libressl / /rootfs
COPY --from=pkg-libseccomp / /rootfs
COPY --from=pkg-linux-firmware /lib/firmware/bnx2 /rootfs/lib/firmware/bnx2
COPY --from=pkg-linux-firmware /lib/firmware/bnx2x /rootfs/lib/firmware/bnx2x
COPY --from=pkg-lvm2 / /rootfs
COPY --from=pkg-libaio / /rootfs
COPY --from=pkg-musl / /rootfs
COPY --from=pkg-open-iscsi / /rootfs
COPY --from=pkg-open-isns / /rootfs
COPY --from=pkg-runc / /rootfs
COPY --from=pkg-socat / /rootfs
COPY --from=pkg-xfsprogs / /rootfs
COPY --from=pkg-util-linux /lib/libblkid.* /rootfs/lib/
COPY --from=pkg-util-linux /lib/libuuid.* /rootfs/lib/
COPY --from=pkg-util-linux /lib/libmount.* /rootfs/lib/
COPY --from=pkg-kmod /usr/lib/libkmod.* /rootfs/lib/
COPY --from=pkg-kernel /lib/modules /rootfs/lib/modules
COPY --from=machined /machined /rootfs/sbin/init
COPY --from=apid-image /apid.tar /rootfs/usr/images/
COPY --from=bootkube-image /bootkube.tar /rootfs/usr/images/
COPY --from=timed-image /timed.tar /rootfs/usr/images/
COPY --from=trustd-image /trustd.tar /rootfs/usr/images/
COPY --from=networkd-image /networkd.tar /rootfs/usr/images/
COPY --from=routerd-image /routerd.tar /rootfs/usr/images/
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
COPY ./hack/cleanup.sh /toolchain/bin/cleanup.sh
RUN cleanup.sh /rootfs
COPY hack/containerd.toml /rootfs/etc/cri/containerd.toml
RUN touch /rootfs/etc/resolv.conf
RUN touch /rootfs/etc/hosts
RUN touch /rootfs/etc/os-release
RUN mkdir -pv /rootfs/{boot,usr/local/share,mnt,system}
RUN mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni,usr/libexec/kubernetes}
RUN ln -s /etc/ssl /rootfs/etc/pki
RUN ln -s /etc/ssl /rootfs/usr/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
RUN ln -s /etc/ssl /rootfs/etc/ca-certificates

FROM rootfs-base AS rootfs-squashfs
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
ARG TARGETARCH
COPY --from=initramfs-archive /initramfs.xz /initramfs-${TARGETARCH}.xz

# The talos target generates a docker image that can be used to run Talos
# in containers.

FROM scratch AS talos
COPY --from=rootfs / /
ENTRYPOINT ["/sbin/init"]

# The installer target generates an image that can be used to install Talos to
# various environments.

FROM base AS installer-build
ARG SHA
ARG TAG
ARG PKGS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
WORKDIR /src/cmd/installer
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${IMAGES_PKGS}.Registry=${REGISTRY}" -o /installer
RUN chmod +x /installer

FROM alpine:3.11 AS installer
RUN apk add --no-cache --update \
    bash \
    ca-certificates \
    cdrkit \
    efibootmgr \
    qemu-img \
    util-linux \
    xfsprogs
COPY --from=pkg-grub / /
ARG TARGETARCH
COPY --from=kernel /vmlinuz-${TARGETARCH} /usr/install/vmlinuz
COPY --from=initramfs /initramfs-${TARGETARCH}.xz /usr/install/initramfs.xz
COPY --from=installer-build /installer /bin/installer
RUN ln -s /bin/installer /bin/talosctl
ARG TAG
ENV VERSION ${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
ENTRYPOINT ["/bin/installer"]
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

FROM base AS unit-tests-race
RUN unlink /etc/ssl
COPY --from=rootfs / /
COPY hack/golang/test.sh /bin
ARG TESTPKGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache/go-build /bin/test.sh --race ${TESTPKGS}

# The integration-test target builds integration test binary.

FROM base AS integration-test-linux-build
ARG SHA
ARG TAG
ARG PKGS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go test -c \
    -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Registry=${REGISTRY}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-linux
COPY --from=integration-test-linux-build /src/integration.test /integration-test-linux-amd64

FROM base AS integration-test-darwin-build
ARG SHA
ARG TAG
ARG PKGS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
RUN --mount=type=cache,target=/.cache/go-build GOOS=darwin GOARCH=amd64 go test -c \
    -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Registry=${REGISTRY}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-darwin
COPY --from=integration-test-darwin-build /src/integration.test /integration-test-darwin-amd64

# The integration-test-provision target builds integration test binary with provisioning tests.

FROM base AS integration-test-provision-linux-build
ARG SHA
ARG TAG
ARG PKGS
ARG USERNAME
ARG REGISTRY
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG IMAGES_PKGS="github.com/talos-systems/talos/pkg/images"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
ARG ARTIFACTS
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go test -c \
    -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${VERSION_PKG}.PkgsVersion=${PKGS} -X ${IMAGES_PKGS}.Username=${USERNAME} -X ${IMAGES_PKGS}.Registry=${REGISTRY} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" \
    -tags integration,integration_provision \
    ./internal/integration

FROM scratch AS integration-test-provision-linux
COPY --from=integration-test-provision-linux-build /src/integration.test /integration-test-provision-linux-amd64

# The lint target performs linting on the source code.

FROM base AS lint-go
COPY .golangci.yml .
ENV GOGC=50
RUN --mount=type=cache,target=/.cache/go-build --mount=type=cache,target=/.cache/golangci-lint golangci-lint run --config .golangci.yml
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache/go-build --mount=type=cache,target=/.cache/golangci-lint golangci-lint run --config ../../.golangci.yml
WORKDIR /src
RUN --mount=type=cache,target=/.cache/go-build importvet github.com/talos-systems/talos/...
RUN find . -name '*.pb.go' | xargs rm
RUN FILES="$(gofumports -l -local github.com/talos-systems/talos .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumports -w -local github.com/talos-systems/talos .':\n${FILES}"; exit 1)

# The protolint target performs linting on protobuf files.

FROM base AS lint-protobuf
WORKDIR /src/api
COPY api .
COPY prototool.yaml .
RUN prototool lint --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include

# The markdownlint target performs linting on Markdown files.

FROM node:14.5.0-alpine AS lint-markdown
RUN npm i -g markdownlint-cli@0.23.2
RUN npm i -g textlint@11.7.6
RUN npm i -g textlint-rule-one-sentence-per-line@1.0.2
WORKDIR /src
COPY .markdownlint.json .
COPY . .
RUN markdownlint --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' .
RUN find . -name '*.md' -not -path '*/node_modules/*' -not -path '*/docs/talosctl/*' | xargs textlint --rule one-sentence-per-line --stdin-filename

# The docs target generates documentation.

FROM base AS docs-build
WORKDIR /src/pkg/machinery/config
RUN go generate ./types/v1alpha1
WORKDIR /src
COPY --from=talosctl-linux /talosctl-linux-amd64 /bin/talosctl
RUN mkdir -p /docs/talosctl \
    && env HOME=/home/user TAG=latest /bin/talosctl docs /docs/talosctl

FROM scratch AS docs
COPY --from=docs-build /tmp/v1alpha1.md /docs/website/content/v0.7/en/configuration/v1alpha1.md
COPY --from=docs-build /docs/talosctl/* /docs/talosctl/
