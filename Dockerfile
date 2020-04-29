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
RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | bash -s -- -b /toolchain/bin v1.24.0
RUN cd $(mktemp -d) \
    && go mod init tmp \
    && go get mvdan.cc/gofumpt/gofumports@aaa7156f4122b1055c466e26e77812fa32bac1d9 \
    && mv /go/bin/gofumports /toolchain/go/bin/gofumports
RUN curl -sfL https://github.com/uber/prototool/releases/download/v1.8.0/prototool-Linux-x86_64.tar.gz | tar -xz --strip-components=2 -C /toolchain/bin prototool/bin/prototool
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
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api common/common.proto
COPY ./api/health/health.proto /api/health/health.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api health/health.proto
COPY ./api/os/os.proto /api/os/os.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api os/os.proto
COPY ./api/security/security.proto /api/security/security.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api security/security.proto
COPY ./api/machine/machine.proto /api/machine/machine.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api machine/machine.proto
COPY ./api/time/time.proto /api/time/time.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api time/time.proto
COPY ./api/network/network.proto /api/network/network.proto
RUN protoc -I/api --go_out=plugins=grpc,paths=source_relative:/api network/network.proto
# Gofumports generated files to adjust import order
RUN gofumports -w -local github.com/talos-systems/talos /api/

FROM scratch AS generate
COPY --from=generate-build /api/common/common.pb.go /api/common/
COPY --from=generate-build /api/health/health.pb.go /api/health/
COPY --from=generate-build /api/os/os.pb.go /api/os/
COPY --from=generate-build /api/security/security.pb.go /api/security/
COPY --from=generate-build /api/machine/machine.pb.go /api/machine/
COPY --from=generate-build /api/time/time.pb.go /api/time/
COPY --from=generate-build /api/network/network.pb.go /api/network/

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

# The timed target builds the timed image.

FROM base AS timed-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/timed
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /timed
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
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/apid
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /apid
RUN chmod +x /apid

FROM base AS apid-image
ARG TAG
ARG USERNAME
COPY --from=apid-build /apid /scratch/apid
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./apid /apid\nENTRYPOINT [\"/apid\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/apid:${TAG} --output type=docker,dest=/apid.tar --no-console  .

# The osd target builds the osd image.

FROM base AS osd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/osd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /osd
RUN chmod +x /osd

FROM base AS osd-image
ARG TAG
ARG USERNAME
COPY --from=osd-build /osd /scratch/osd
WORKDIR /scratch
RUN printf "FROM scratch\nCOPY ./osd /osd\nENTRYPOINT [\"/osd\"]" > Dockerfile
RUN --security=insecure img build --tag ${USERNAME}/osd:${TAG} --output type=docker,dest=/osd.tar --no-console  .

# The trustd target builds the trustd image.

FROM base AS trustd-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/internal/app/trustd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /trustd
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
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/networkd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /networkd
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
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/routerd
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /routerd
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
ARG VERSION_PKG="github.com/talos-systems/talos/internal/pkg/version"
WORKDIR /src/internal/app/bootkube
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Server -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /bootkube
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
ARG ARTIFACTS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-linux-amd64
RUN chmod +x /talosctl-linux-amd64

FROM base AS talosctl-linux-arm64-build
ARG SHA
ARG TAG
ARG ARTIFACTS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=arm64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-linux-arm64
RUN chmod +x /talosctl-linux-arm64

FROM base AS talosctl-linux-armv7-build
ARG SHA
ARG TAG
ARG ARTIFACTS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=arm GOARM=7  go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-linux-armv7
RUN chmod +x /talosctl-linux-armv7

FROM scratch AS talosctl-linux
COPY --from=talosctl-linux-amd64-build /talosctl-linux-amd64 /talosctl-linux-amd64
COPY --from=talosctl-linux-arm64-build /talosctl-linux-arm64 /talosctl-linux-arm64
COPY --from=talosctl-linux-armv7-build /talosctl-linux-armv7 /talosctl-linux-armv7

FROM base AS talosctl-darwin-build
ARG SHA
ARG TAG
ARG ARTIFACTS
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
WORKDIR /src/cmd/talosctl
RUN --mount=type=cache,target=/.cache/go-build GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" -o /talosctl-darwin-amd64
RUN chmod +x /talosctl-darwin-amd64

FROM scratch AS talosctl-darwin
COPY --from=talosctl-darwin-build /talosctl-darwin-amd64 /talosctl-darwin-amd64

# The kernel target is the linux kernel.

FROM scratch AS kernel
COPY --from=docker.io/autonomy/kernel:v0.2.0 /boot/vmlinuz /vmlinuz
COPY --from=docker.io/autonomy/kernel:v0.2.0 /boot/vmlinux /vmlinux

# The rootfs target provides the Talos rootfs.

FROM build AS rootfs-base
COPY --from=docker.io/autonomy/fhs:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/ca-certificates:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/containerd:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/dosfstools:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/eudev:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/iptables:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/libressl:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/libseccomp:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/linux-firmware:v0.2.0 /lib/firmware/bnx2 /rootfs/lib/firmware/bnx2
COPY --from=docker.io/autonomy/linux-firmware:v0.2.0 /lib/firmware/bnx2x /rootfs/lib/firmware/bnx2x
COPY --from=docker.io/autonomy/musl:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/runc:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/socat:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/syslinux:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/xfsprogs:v0.2.0 / /rootfs
COPY --from=docker.io/autonomy/util-linux:v0.2.0 /lib/libblkid.* /rootfs/lib
COPY --from=docker.io/autonomy/util-linux:v0.2.0 /lib/libuuid.* /rootfs/lib
COPY --from=docker.io/autonomy/kmod:v0.2.0 /usr/lib/libkmod.* /rootfs/lib
COPY --from=docker.io/autonomy/kernel:v0.2.0 /lib/modules /rootfs/lib/modules
COPY --from=machined /machined /rootfs/sbin/init
COPY --from=apid-image /apid.tar /rootfs/usr/images/
COPY --from=bootkube-image /bootkube.tar /rootfs/usr/images/
COPY --from=timed-image /timed.tar /rootfs/usr/images/
COPY --from=osd-image /osd.tar /rootfs/usr/images/
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
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
WORKDIR /src/cmd/installer
RUN --mount=type=cache,target=/.cache/go-build go build -ldflags "-s -w -X ${VERSION_PKG}.Name=Talos -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" -o /installer
RUN chmod +x /installer

FROM alpine:3.8 AS installer
RUN apk add --no-cache --update \
    bash \
    ca-certificates \
    cdrkit \
    qemu-img \
    syslinux \
    util-linux \
    xfsprogs
COPY --from=kernel /vmlinuz /usr/install/vmlinuz
COPY --from=rootfs /usr/lib/syslinux/ /usr/lib/syslinux
COPY --from=initramfs /initramfs.xz /usr/install/initramfs.xz
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

FROM golang:${GO_VERSION} AS unit-tests-race
COPY --from=base /src /src
COPY --from=base /go/pkg/mod /go/pkg/mod
WORKDIR /src
ENV GO111MODULE on
ARG TESTPKGS
RUN --mount=type=cache,target=/root/.cache/go-build go test -v -count 1 -race ${TESTPKGS}

# The integration-test target builds integration test binary.

FROM base AS integration-test-linux-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go test -c \
    -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-linux
COPY --from=integration-test-linux-build /src/integration.test /integration-test-linux-amd64

FROM base AS integration-test-darwin-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
RUN --mount=type=cache,target=/.cache/go-build GOOS=darwin GOARCH=amd64 go test -c \
    -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-darwin
COPY --from=integration-test-darwin-build /src/integration.test /integration-test-darwin-amd64

# The integration-test-provision target builds integration test binary with provisioning tests.

FROM base AS integration-test-provision-linux-build
ARG SHA
ARG TAG
ARG VERSION_PKG="github.com/talos-systems/talos/pkg/version"
ARG MGMT_HELPERS_PKG="github.com/talos-systems/talos/cmd/talosctl/pkg/mgmt/helpers"
ARG ARTIFACTS
RUN --mount=type=cache,target=/.cache/go-build GOOS=linux GOARCH=amd64 go test -c \
    -ldflags "-s -w -X ${VERSION_PKG}.Name=Client -X ${VERSION_PKG}.SHA=${SHA} -X ${VERSION_PKG}.Tag=${TAG} -X ${MGMT_HELPERS_PKG}.ArtifactsPath=${ARTIFACTS}" \
    -tags integration,integration_provision \
    ./internal/integration

FROM scratch AS integration-test-provision-linux
COPY --from=integration-test-provision-linux-build /src/integration.test /integration-test-provision-linux-amd64

# The lint target performs linting on the source code.

FROM base AS lint-go
COPY .golangci.yml .
ENV GOGC=50
RUN --mount=type=cache,target=/.cache/go-build golangci-lint run --config .golangci.yml
RUN find . -name '*.pb.go' | xargs rm
RUN FILES="$(gofumports -l -local github.com/talos-systems/talos .)" && test -z "${FILES}" || (echo -e "Source code is not formatted with 'gofumports -w -local github.com/talos-systems/talos .':\n${FILES}"; exit 1)

# The protolint target performs linting on protobuf files.

FROM base AS lint-protobuf
WORKDIR /src/api
COPY api .
COPY prototool.yaml .
RUN prototool lint --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include

# The markdownlint target performs linting on Markdown files.

FROM node:8.16.1-alpine AS lint-markdown
RUN npm i -g markdownlint-cli
RUN npm i -g textlint
RUN npm i -g textlint-rule-one-sentence-per-line
WORKDIR /src
COPY .markdownlint.json .
COPY . .
RUN markdownlint --ignore "**/node_modules/**" --ignore '**/hack/chglog/**' .
RUN find . -name '*.md' -not -path '*/node_modules/*' -not -path '*/docs/talosctl/*' | xargs textlint --rule one-sentence-per-line --stdin-filename

# The docs target generates documentation.

FROM base AS docs-build
RUN go generate ./pkg/config/types/v1alpha1
COPY --from=talosctl-linux /talosctl-linux-amd64 /bin/talosctl
RUN mkdir -p /docs/talosctl \
    && env HOME=/home/user TAG=latest /bin/talosctl docs /docs/talosctl

FROM scratch AS docs
COPY --from=docs-build /tmp/v1alpha1.md /docs/website/content/v0.4/en/configuration/v1alpha1.md
COPY --from=docs-build /docs/talosctl/* /docs/talosctl/
