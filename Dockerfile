# syntax=docker/dockerfile:experimental
ARG KERNEL_IMAGE
ARG TOOLCHAIN_IMAGE

# The proto target will generate code based on service definitions.

ARG TOOLCHAIN_IMAGE
FROM ${TOOLCHAIN_IMAGE} AS proto-build
WORKDIR /osd
COPY ./internal/app/osd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /trustd
COPY ./internal/app/trustd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /blockd
COPY ./internal/app/blockd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto

FROM scratch AS proto
COPY --from=proto-build /osd/proto/api.pb.go /internal/app/osd/proto/
COPY --from=proto-build /trustd/proto/api.pb.go /internal/app/trustd/proto/
COPY --from=proto-build /blockd/proto/api.pb.go /internal/app/blockd/proto/

# The base target provides a common starting point for all other targets.

ARG TOOLCHAIN_IMAGE
FROM ${TOOLCHAIN_IMAGE} AS base
# ca-certificates
RUN mkdir -p /etc/ssl/certs
RUN ln -s /toolchain/etc/ssl/certs/ca-certificates /etc/ssl/certs/ca-certificates
# fhs
COPY hack/scripts/fhs.sh /bin
RUN fhs.sh /rootfs
# ca-certificates
RUN mkdir -p /rootfs/etc/ssl/certs
RUN curl -o /rootfs/etc/ssl/certs/ca-certificates.crt https://curl.haxx.se/ca/cacert.pem
# xfsprogs
WORKDIR /tmp/xfsprogs
RUN curl -L https://www.kernel.org/pub/linux/utils/fs/xfs/xfsprogs/xfsprogs-4.18.0.tar.xz | tar -xJ --strip-components=1
RUN make \
    DEBUG=-DNDEBUG \
    INSTALL_USER=0 \
    INSTALL_GROUP=0 \
    LOCAL_CONFIGURE_OPTIONS="--prefix=/"
RUN make install DESTDIR=/rootfs
WORKDIR /tmp/dosfstools
RUN curl -L https://github.com/dosfstools/dosfstools/releases/download/v4.1/dosfstools-4.1.tar.xz | tar -xJ --strip-components=1
RUN ./configure \
    --prefix=/ \
    --enable-compat-symlinks
RUN make
RUN make install DESTDIR=/rootfs
# libblkid
RUN cp /toolchain/lib/libblkid.* /rootfs/lib
# libuuid
RUN cp /toolchain/lib/libuuid.* /rootfs/lib
# syslinux/extlinux
RUN mkdir /tmp/syslinux
WORKDIR /tmp/syslinux
RUN curl -L https://www.kernel.org/pub/linux/utils/boot/syslinux/syslinux-6.03.tar.xz | tar -xJ --strip-components=1
RUN ln -s /toolchain/bin/pwd /bin/pwd && \
    make installer && \
    cp /tmp/syslinux/bios/extlinux/extlinux /rootfs/bin && \
    cp /tmp/syslinux/efi64/mbr/gptmbr.bin /rootfs/share
# golang
ENV GOPATH /toolchain/gopath
RUN mkdir -p ${GOPATH}
# context
ENV GO111MODULE on
ENV CGO_ENABLED 0
WORKDIR /src
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY ./go.mod ./
COPY ./go.sum ./
COPY --from=proto /internal/app ./internal/app
RUN go mod download
RUN go mod verify

# The udevd target builds the udevd binary.

FROM base AS udevd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/udevd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /udevd
RUN chmod +x /udevd
ARG APP
FROM scratch AS udevd
COPY --from=udevd-build /udevd /udevd
ENTRYPOINT ["/udevd"]

# The kernel target is the linux kernel.

ARG KERNEL_IMAGE
FROM ${KERNEL_IMAGE} as kernel

# The initramfs target creates an initramfs.

FROM base AS initramfs-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/init
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /init
RUN chmod +x /init
WORKDIR /initramfs
RUN cp /init ./
COPY --from=base /rootfs ./
WORKDIR /src
COPY hack/scripts/cleanup.sh /bin
RUN cleanup.sh /initramfs
# bootloader
COPY --from=base /rootfs/share/gptmbr.bin /initramfs/usr/share/gptmbr.bin
COPY --from=base /rootfs/bin/extlinux /initramfs/bin/extlinux
WORKDIR /initramfs
RUN set -o pipefail && find . 2>/dev/null | cpio -H newc -o | xz -v -C crc32 -0 -e -T 0 -z >/initramfs.xz
FROM scratch AS initramfs
COPY --from=initramfs-build /initramfs.xz /initramfs.xz

# The rootfs target creates a root filesysyem with only what is required to run
# Kubernetes.

FROM base AS rootfs-build
# iptables
WORKDIR /toolchain/usr/local/src/iptables
RUN curl -L http://www.netfilter.org/projects/iptables/files/iptables-1.8.2.tar.bz2 | tar --strip-components=1 -xj
WORKDIR /toolchain/usr/local/src/iptables/build
RUN ../configure \
     --prefix=/usr \
     --libexecdir=/usr/libexec \
     --disable-static \
     --sbindir=/sbin \
     --disable-nftables \
     --enable-libipq \
     --with-xtlibdir=/lib/xtables
RUN make
RUN make install DESTDIR=/rootfs
# libseccomp
WORKDIR /toolchain/usr/local/src/libseccomp
RUN curl -L https://github.com/seccomp/libseccomp/releases/download/v2.3.3/libseccomp-2.3.3.tar.gz | tar --strip-components=1 -xz
WORKDIR /toolchain/usr/local/src/libseccomp/build
RUN ../configure \
    --prefix=/usr \
    --disable-static
RUN make -j $(($(nproc) / 2))
RUN make install DESTDIR=/rootfs
RUN make install DESTDIR=/toolchain
# crictl
RUN curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-amd64.tar.gz | tar -xz -C /rootfs/bin
# containerd
RUN mkdir -p $GOPATH/src/github.com/containerd \
    && cd $GOPATH/src/github.com/containerd \
    && git clone https://github.com/containerd/containerd.git \
    && cd $GOPATH/src/github.com/containerd/containerd \
    && git checkout v1.2.5
RUN cd $GOPATH/src/github.com/containerd/containerd \
    && make binaries BUILDTAGS=no_btrfs \
    && cp bin/containerd /rootfs/bin \
    && cp bin/containerd-shim /rootfs/bin
# runc
RUN curl -L https://github.com/opencontainers/runc/releases/download/v1.0.0-rc6/runc.amd64 -o /rootfs/bin/runc
RUN chmod +x /rootfs/bin/runc
# CNI
RUN mkdir -p /rootfs/opt/cni/bin
RUN curl -L https://github.com/containernetworking/cni/releases/download/v0.6.0/cni-amd64-v0.6.0.tgz | tar -xz -C /rootfs/opt/cni/bin
RUN curl -L https://github.com/containernetworking/plugins/releases/download/v0.7.4/cni-plugins-amd64-v0.7.4.tgz | tar -xz -C /rootfs/opt/cni/bin
# kubeadm
RUN curl --retry 3 --retry-delay 60 -L https://storage.googleapis.com/kubernetes-release/release/v1.14.0/bin/linux/amd64/kubeadm -o /rootfs/bin/kubeadm
RUN chmod +x /rootfs/bin/kubeadm
# udevd
COPY --from=udevd-build /udevd /rootfs/bin/udevd
# images
COPY images /rootfs/usr/images
# cleanup
WORKDIR /src
COPY hack/scripts/cleanup.sh /bin
RUN cleanup.sh /rootfs
COPY hack/scripts/symlink.sh /bin
RUN symlink.sh /rootfs
# bootloader
COPY --from=base /rootfs/share/gptmbr.bin /rootfs/usr/share/gptmbr.bin
WORKDIR /rootfs
RUN ["/toolchain/bin/tar", "-cvpzf", "/rootfs.tar.gz", "."]
FROM scratch AS rootfs
COPY --from=rootfs-build /rootfs.tar.gz /rootfs.tar.gz

# The talos target generates a docker image that can be used to run Talos
# in containers.

FROM rootfs-build AS docker-rootfs-build
# workaround docker overwriting our /etc symlink
RUN rm /rootfs/etc
RUN mv /rootfs/var/etc /rootfs/etc
RUN ln -s /etc /rootfs/var/etc
FROM scratch AS talos
COPY --from=docker-rootfs-build /rootfs /
COPY --from=initramfs-build /init /sbin/init
ENTRYPOINT ["/sbin/init"]

# The installer target generates an image that can be used to install Talos to
# various environments.

FROM alpine:3.8 AS installer
RUN apk --update add bash curl gzip e2fsprogs tar cdrkit parted syslinux util-linux xfsprogs xz sgdisk sfdisk qemu-img unzip
WORKDIR /usr/local/src/syslinux
RUN curl -L https://www.kernel.org/pub/linux/utils/boot/syslinux/syslinux-6.03.tar.xz | tar --strip-components=1 -xJ
WORKDIR /
COPY --from=kernel /vmlinuz /generated/boot/vmlinuz
COPY --from=rootfs /rootfs.tar.gz /generated/rootfs.tar.gz
COPY --from=initramfs /initramfs.xz /generated/boot/initramfs.xz
RUN curl -L https://releases.hashicorp.com/packer/1.3.1/packer_1.3.1_linux_amd64.zip -o /tmp/packer.zip \
    && unzip -d /tmp /tmp/packer.zip \
    && mv /tmp/packer /bin \
    && rm /tmp/packer.zip
COPY hack/installer/packer.json /packer.json
COPY hack/installer/entrypoint.sh /bin/entrypoint.sh
ARG TAG
ENV VERSION ${TAG}
ENTRYPOINT ["entrypoint.sh"]

# The test target performs tests on the codebase.

FROM base AS test-runner
# xfsprogs is required by the tests
ENV PATH /rootfs/bin:$PATH
COPY hack/golang/test.sh /bin
RUN test.sh --short
RUN test.sh
FROM scratch AS test
COPY --from=test-runner /src/coverage.txt /coverage.txt

# The lint target performs linting on the codebase.

FROM base AS lint
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /toolchain/bin v1.14.0
COPY hack/golang/golangci-lint.yaml .
RUN golangci-lint run --config golangci-lint.yaml
#
# The osd target builds the osd binary.

FROM base AS osd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/osd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osd
RUN chmod +x /osd
ARG APP
FROM scratch AS osd
COPY --from=osd-build /osd /osd
ENTRYPOINT ["/osd"]

# The osctl target builds the osctl binaries.

FROM base AS osctl-linux-amd64-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/cmd/osctl
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -a -ldflags "-s -w -linkmode external -extldflags \"-static\" -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-linux-amd64
RUN chmod +x /osctl-linux-amd64
FROM scratch AS osctl-linux-amd64
COPY --from=osctl-linux-amd64-build /osctl-linux-amd64 /osctl-linux-amd64

FROM base AS osctl-darwin-amd64-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/cmd/osctl
RUN GOOS=darwin GOARCH=amd64 go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-darwin-amd64
RUN chmod +x /osctl-darwin-amd64
FROM scratch AS osctl-darwin-amd64
COPY --from=osctl-darwin-amd64-build /osctl-darwin-amd64 /osctl-darwin-amd64

# The trustd target builds the trustd binary.

FROM base AS trustd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/trustd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /trustd
RUN chmod +x /trustd
ARG APP
FROM scratch AS trustd
COPY --from=trustd-build /trustd /trustd
ENTRYPOINT ["/trustd"]

# The proxyd target builds the proxyd binaries.

FROM base AS proxyd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/proxyd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /proxyd
RUN chmod +x /proxyd
ARG APP
FROM scratch AS proxyd
COPY --from=proxyd-build /proxyd /proxyd
ENTRYPOINT ["/proxyd"]

# The blockd target builds the blockd binaries.

FROM base AS blockd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/blockd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /blockd
RUN chmod +x /blockd
ARG APP
FROM scratch AS blockd
COPY --from=blockd-build /blockd /blockd
ENTRYPOINT ["/blockd"]

# The osinstall target builds the installer binaries.

FROM base AS osinstall-linux-amd64-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/osinstall
RUN GOOS=linux GOARCH=amd64 go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osinstall-linux-amd64
RUN chmod +x /osinstall-linux-amd64
FROM scratch AS osinstall-linux-amd64
COPY --from=osinstall-linux-amd64-build /osinstall-linux-amd64 /osinstall-linux-amd64

# The ntpd target builds the ntpd binary
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
