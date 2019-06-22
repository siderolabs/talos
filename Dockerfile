ARG KERNEL_IMAGE
ARG TOOLCHAIN_IMAGE
ARG ROOTFS_IMAGE
ARG INITRAMFS_IMAGE

# The proto target generates code from protobuf service definitions.

ARG TOOLCHAIN_IMAGE
FROM ${TOOLCHAIN_IMAGE} AS proto-build
WORKDIR /osd
COPY ./internal/app/osd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /trustd
COPY ./internal/app/trustd/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto
WORKDIR /init
COPY ./internal/app/init/proto ./proto
RUN protoc -I/usr/local/include -I./proto --go_out=plugins=grpc:proto proto/api.proto

FROM scratch AS proto
COPY --from=proto-build /osd/proto/api.pb.go /internal/app/osd/proto/
COPY --from=proto-build /trustd/proto/api.pb.go /internal/app/trustd/proto/
COPY --from=proto-build /init/proto/api.pb.go /internal/app/init/proto/

# The base provides a common image to build the Talos source code.

ARG TOOLCHAIN_IMAGE
FROM ${TOOLCHAIN_IMAGE} AS base
ENV GOPATH /toolchain/gopath
RUN mkdir -p ${GOPATH}
ENV GO111MODULE on
ENV GOPROXY https://proxy.golang.org
ENV CGO_ENABLED 0
WORKDIR /src
COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download
RUN go mod verify
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY --from=proto /internal/app ./internal/app
RUN go list -mod=readonly all >/dev/null
RUN ! go mod tidy -v 2>&1 | grep .

# The osd target builds the osd binary.

FROM base AS osd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/osd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osd
RUN chmod +x /osd

FROM scratch AS osd
COPY --from=osd-build /osd /osd
ENTRYPOINT ["/osd"]

# The osctl targets build the osctl binaries.

FROM base AS osctl-linux-amd64-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/cmd/osctl
RUN GOOS=linux GOARCH=amd64 go build -a -ldflags "-s -w -linkmode external -extldflags \"-static\" -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osctl-linux-amd64
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

# The trustd target builds the trustd image.

FROM base AS trustd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/trustd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /trustd
RUN chmod +x /trustd

FROM scratch AS trustd
COPY --from=trustd-build /trustd /trustd
ENTRYPOINT ["/trustd"]

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

# The udevd target builds the udevd image.

FROM base AS udevd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/udevd
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /udevd
RUN chmod +x /udevd

FROM scratch AS udevd
COPY --from=udevd-build /udevd /udevd
ENTRYPOINT ["/udevd"]

# The binaries target allows for parallel compilation of all binaries.

FROM scratch AS binaries-build
COPY --from=init / /
COPY --from=osd / /
COPY --from=trustd / /
COPY --from=proxyd / /
COPY --from=ntpd / /
COPY --from=udevd / /
COPY --from=osctl-linux-amd64 / /
COPY --from=osctl-darwin-amd64 / /

FROM scratch AS binaries
COPY --from=binaries-build /osctl-linux-amd64 /osctl-linux-amd64
COPY --from=binaries-build /osctl-darwin-amd64 /osctl-darwin-amd64

# The kernel target is the linux kernel.

ARG KERNEL_IMAGE
FROM ${KERNEL_IMAGE} as kernel

# The initramfs target creates the compressed initramfs.

FROM base AS init-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/init
RUN go build -a -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /init
RUN chmod +x /init

FROM scratch AS init
COPY --from=init-build /init /init

ARG INITRAMFS_IMAGE
FROM ${INITRAMFS_IMAGE} AS initramfs-build
WORKDIR /
COPY --from=init-build /init /init

ARG TOOLCHAIN_IMAGE
FROM ${TOOLCHAIN_IMAGE} AS initramfs-archive
COPY --from=initramfs-build / /initramfs
WORKDIR /initramfs
RUN set -o pipefail && find . 2>/dev/null | cpio -H newc -o | xz -v -C crc32 -0 -e -T 0 -z >/initramfs.xz

FROM scratch AS initramfs
COPY --from=initramfs-archive /initramfs.xz /initramfs.xz

# The rootfs target creates the root filesystem archive.

ARG ROOTFS_IMAGE
FROM ${ROOTFS_IMAGE} AS rootfs-build
COPY --from=kernel /modules /lib/modules
COPY images /usr/images

ARG TOOLCHAIN_IMAGE
FROM ${TOOLCHAIN_IMAGE} AS rootfs-archive
COPY --from=rootfs-build / /rootfs
WORKDIR /rootfs
RUN tar -cvpzf /rootfs.tar.gz .

FROM scratch AS rootfs
COPY --from=rootfs-archive /rootfs.tar.gz /rootfs.tar.gz

# The test target performs tests on the source code.

FROM base AS test
COPY --from=rootfs-build / /rootfs
ENV PATH /rootfs/bin:$PATH
COPY hack/golang/test.sh /bin

# The lint target performs linting on the codebase.

FROM base AS lint
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /toolchain/bin v1.16.0
COPY hack/golang/golangci-lint.yaml .
RUN golangci-lint run --config golangci-lint.yaml

# The talos target generates a docker image that can be used to run Talos
# in containers.

ARG TOOLCHAIN_IMAGE
FROM ${TOOLCHAIN_IMAGE} AS talos-build
COPY --from=rootfs-build / /rootfs
# A workaround docker overwriting our /etc symlink.
RUN rm /rootfs/etc
RUN mv /rootfs/var/etc /rootfs/etc
RUN ln -s /etc /rootfs/var/etc

FROM scratch AS talos
COPY --from=talos-build /rootfs /
COPY --from=init-build /init /init
ENTRYPOINT ["/init"]

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
COPY --from=kernel /vmlinuz /usr/install/vmlinuz
COPY --from=initramfs /initramfs.xz /usr/install/initramfs.xz
COPY --from=rootfs /rootfs.tar.gz /usr/install/rootfs.tar.gz
COPY --from=initramfs-build /usr/lib/syslinux/ /usr/lib/syslinux
COPY --from=osctl-linux-amd64-build /osctl-linux-amd64 /bin/osctl
RUN curl -L https://releases.hashicorp.com/packer/1.3.1/packer_1.3.1_linux_amd64.zip -o /tmp/packer.zip \
    && unzip -d /tmp /tmp/packer.zip \
    && mv /tmp/packer /bin \
    && rm /tmp/packer.zip
COPY hack/installer/packer.json /packer.json
COPY hack/installer/entrypoint.sh /bin/entrypoint.sh
ARG TAG
ENV VERSION ${TAG}
ENTRYPOINT ["entrypoint.sh"]
