# syntax=docker/dockerfile:experimental
ARG TOOLCHAIN_VERSION
ARG KERNEL_VERSION
ARG GOLANG_VERSION

ARG GOLANG_VERSION
FROM golang:${GOLANG_VERSION} AS proto
RUN apt update
RUN apt -y install bsdtar
WORKDIR /go/src/github.com/golang/protobuf
RUN curl -L https://github.com/golang/protobuf/archive/v1.2.0.tar.gz | tar -xz --strip-components=1
RUN cd protoc-gen-go && go install .
RUN curl -L https://github.com/google/protobuf/releases/download/v3.6.1/protoc-3.6.1-linux-x86_64.zip | bsdtar -xf - -C /tmp \
    && mv /tmp/bin/protoc /bin \
    && mv /tmp/include/* /usr/local/include \
    && chmod +x /bin/protoc
WORKDIR /osd
COPY ./internal/app/osd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /trustd
COPY ./internal/app/trustd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /blockd
COPY ./internal/app/blockd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto

ARG GOLANG_VERSION
FROM golang:${GOLANG_VERSION} AS base
ENV GO111MODULE on
WORKDIR /src
COPY ./ ./
ENV GOOS linux
ENV GOARCH amd64
ENV CGO_ENABLED 0
COPY --from=proto /osd/proto/api.pb.go ./internal/app/osd/proto
COPY --from=proto /trustd/proto/api.pb.go ./internal/app/trustd/proto
COPY --from=proto /blockd/proto/api.pb.go ./internal/app/blockd/proto
RUN go mod download
RUN go mod verify

FROM base AS udevd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/autonomy/talos/internal/pkg/version"
WORKDIR /src/internal/app/udevd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /udevd
RUN chmod +x /udevd
ARG APP
FROM scratch AS udevd
COPY --from=udevd-build /udevd /udevd
ENTRYPOINT ["/udevd"]

ARG TOOLCHAIN_VERSION
FROM autonomy/toolchain:${TOOLCHAIN_VERSION} AS common
RUN rm -rf /rootfs/etc
RUN mkdir -p /rootfs/var/etc
RUN ln -sv var/etc /rootfs/etc
# xfsprogs
RUN mkdir -p /etc/ssl/certs
RUN ln -s /toolchain/etc/ssl/certs/ca-certificates /etc/ssl/certs/ca-certificates
WORKDIR /tmp/xfsprogs
RUN curl -L https://www.kernel.org/pub/linux/utils/fs/xfs/xfsprogs/xfsprogs-4.18.0.tar.xz | tar -xJ --strip-components=1
RUN make \
    DEBUG=-DNDEBUG \
    INSTALL_USER=0 \
    INSTALL_GROUP=0 \
    LOCAL_CONFIGURE_OPTIONS="--prefix=/"
RUN make install DESTDIR=/rootfs

FROM common AS rootfs
RUN rm -rf /rootfs/etc
RUN mkdir -p /rootfs/var/etc
RUN ln -sv var/etc /rootfs/etc
# libseccomp
WORKDIR /toolchain/usr/local/src/libseccomp
RUN curl -L https://github.com/seccomp/libseccomp/releases/download/v2.3.3/libseccomp-2.3.3.tar.gz | tar --strip-components=1 -xz
WORKDIR /toolchain/usr/local/src/libseccomp/build
RUN ../configure \
    --prefix=/usr \
    --disable-static
RUN make -j $(($(nproc) / 2))
RUN make install DESTDIR=/rootfs
# ca-certificates
RUN mkdir -p /rootfs/etc/ssl/certs
RUN curl -o /rootfs/etc/ssl/certs/ca-certificates.crt https://curl.haxx.se/ca/cacert.pem
# containerd
RUN curl -L https://github.com/kubernetes-sigs/cri-tools/releases/download/v1.13.0/crictl-v1.13.0-linux-amd64.tar.gz | tar -xz -C /rootfs/bin
RUN curl -L https://github.com/containerd/containerd/releases/download/v1.2.1/containerd-1.2.1.linux-amd64.tar.gz | tar --strip-components=1 -xz -C /rootfs/bin
RUN rm /rootfs/bin/ctr
# runc
RUN curl -L https://github.com/opencontainers/runc/releases/download/v1.0.0-rc6/runc.amd64 -o /rootfs/bin/runc
RUN chmod +x /rootfs/bin/runc
RUN ln -sv ../opt /rootfs/var/opt
RUN mkdir -p /rootfs/opt/cni/bin
# CNI
RUN curl -L https://github.com/containernetworking/cni/releases/download/v0.6.0/cni-amd64-v0.6.0.tgz | tar -xz -C /rootfs/opt/cni/bin
RUN curl -L https://github.com/containernetworking/plugins/releases/download/v0.7.4/cni-plugins-amd64-v0.7.4.tgz | tar -xz -C /rootfs/opt/cni/bin
# kubeadm
RUN curl --retry 3 --retry-delay 60 -L https://storage.googleapis.com/kubernetes-release/release/v1.13.1/bin/linux/amd64/kubeadm -o /rootfs/bin/kubeadm
RUN chmod +x /rootfs/bin/kubeadm
# images
COPY images /rootfs/usr/images
# udevd
COPY --from=udevd-build /udevd /rootfs/bin/udevd
# cleanup
COPY ./hack/scripts/cleanup.sh /bin
RUN chmod +x /bin/cleanup.sh
RUN /bin/cleanup.sh /rootfs
COPY ./hack/scripts/symlink.sh /bin
RUN chmod +x /bin/symlink.sh
RUN /bin/symlink.sh /rootfs
WORKDIR /rootfs
RUN ["/toolchain/bin/tar", "-cvpzf", "/rootfs.tar.gz", "."]

FROM base AS initramfs
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/autonomy/talos/internal/pkg/version"
RUN apt update \
    && apt install -y cpio xz-utils
WORKDIR /src/internal/app/init
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /init
RUN chmod +x /init
WORKDIR /initramfs
RUN cp /init ./
COPY --from=common /rootfs ./
COPY ./hack/scripts/cleanup.sh /bin
RUN chmod +x /bin/cleanup.sh
RUN /bin/cleanup.sh /initramfs
RUN find . 2>/dev/null | cpio -H newc -o | xz -v -C crc32 -0 -e -T 0 -z >/initramfs.xz

FROM base AS osd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/autonomy/talos/internal/pkg/version"
WORKDIR /src/internal/app/osd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osd
RUN chmod +x /osd

ARG APP
FROM scratch AS osd
COPY --from=osd-build /osd /osd
ENTRYPOINT ["/osd"]

FROM base AS osctl
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/autonomy/talos/internal/pkg/version"
WORKDIR /src/internal/app/osctl
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-linux-amd64
RUN GOOS=darwin GOARCH=amd64 go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-darwin-amd64
RUN chmod +x /osctl-linux-amd64
RUN chmod +x /osctl-darwin-amd64

FROM base AS trustd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/autonomy/talos/internal/pkg/version"
WORKDIR /src/internal/app/trustd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /trustd
RUN chmod +x /trustd
ARG APP
FROM scratch AS trustd
COPY --from=trustd-build /trustd /trustd
ENTRYPOINT ["/trustd"]

FROM base AS proxyd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/autonomy/talos/internal/pkg/version"
WORKDIR /src/internal/app/proxyd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /proxyd
RUN chmod +x /proxyd
ARG APP
FROM scratch AS proxyd
COPY --from=proxyd-build /proxyd /proxyd
ENTRYPOINT ["/proxyd"]

FROM base AS blockd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/autonomy/talos/internal/pkg/version"
WORKDIR /src/internal/app/blockd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /blockd
RUN chmod +x /blockd
ARG APP
FROM scratch AS blockd
COPY --from=blockd-build /blockd /blockd
ENTRYPOINT ["/blockd"]

FROM base AS test
RUN apt update \
    && apt install -y xfsprogs
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b $GOPATH/bin v1.12.3
RUN chmod +x ./hack/golang/test.sh
ENV PATH /rootfs/bin:$PATH
RUN ./hack/golang/test.sh --unit
RUN ./hack/golang/test.sh --lint ./hack/golang/golangci-lint.yaml

FROM golang:1.11.4 as docs
RUN curl -L https://github.com/gohugoio/hugo/releases/download/v0.49.2/hugo_0.49.2_Linux-64bit.tar.gz | tar -xz -C /bin
WORKDIR /web
COPY ./web ./
RUN mkdir /docs
RUN hugo --destination=/docs --verbose
RUN echo "talos.autonomy.io" > /docs/CNAME

ARG KERNEL_VERSION
FROM autonomy/kernel:${KERNEL_VERSION} as kernel

FROM alpine:3.7 AS installer
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
COPY ./hack/installer/packer.json /packer.json
COPY ./hack/installer/entrypoint.sh /bin/entrypoint.sh
RUN chmod +x /bin/entrypoint.sh
ARG TAG
ENV VERSION ${TAG}
ENTRYPOINT ["entrypoint.sh"]
