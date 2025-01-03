# syntax = docker/dockerfile-upstream:1.12.1-labs

# Meta args applied to stage base names.

ARG TOOLS=scratch
ARG PKGS=scratch
ARG EXTRAS=scratch
ARG INSTALLER_ARCH=scratch
ARG DEBUG_TOOLS_SOURCE=scratch

ARG PKGS_PREFIX=scratch
ARG PKG_FHS=scratch
ARG PKG_CA_CERTIFICATES=scratch
ARG PKG_CRYPTSETUP=scratch
ARG PKG_CONTAINERD=scratch
ARG PKG_DOSFSTOOLS=scratch
ARG PKG_E2FSPROGS=scratch
ARG PKG_SYSTEMD_UDEVD=scratch
ARG PKG_LIBCAP=scratch
ARG PKG_GRUB=scratch
ARG PKG_SD_BOOT=scratch
ARG PKG_IPTABLES=scratch
ARG PKG_IPXE=scratch
ARG PKG_LIBINIH=scratch
ARG PKG_LIBJSON_C=scratch
ARG PKG_LIBMNL=scratch
ARG PKG_LIBNFTNL=scratch
ARG PKG_LIBPOPT=scratch
ARG PKG_LIBSEPOL=scratch
ARG PKG_LIBSELINUX=scratch
ARG PKG_PCRE2=scratch
ARG PKG_LIBURCU=scratch
ARG PKG_OPENSSL=scratch
ARG PKG_LIBSECCOMP=scratch
ARG PKG_LINUX_FIRMWARE=scratch
ARG PKG_LVM2=scratch
ARG PKG_LIBAIO=scratch
ARG PKG_MUSL=scratch
ARG PKG_RUNC=scratch
ARG PKG_XFSPROGS=scratch
ARG PKG_APPARMOR=scratch
ARG PKG_UTIL_LINUX=scratch
ARG PKG_KMOD=scratch
ARG PKG_KERNEL=scratch
ARG PKG_CNI=scratch
ARG PKG_FLANNEL_CNI=scratch
ARG PKG_TALOSCTL_CNI_BUNDLE_INSTALL=scratch

ARG DEBUG_TOOLS_SOURCE=scratch

ARG EMBED_TARGET=embed

# Resolve package images using ${PKGS} to be used later in COPY --from=.

FROM ${PKG_FHS} AS pkg-fhs
FROM ${PKG_CA_CERTIFICATES} AS pkg-ca-certificates

FROM --platform=amd64 ${PKG_APPARMOR} AS pkg-apparmor-amd64
FROM --platform=arm64 ${PKG_APPARMOR} AS pkg-apparmor-arm64

FROM --platform=amd64 ${PKG_CRYPTSETUP} AS pkg-cryptsetup-amd64
FROM --platform=arm64 ${PKG_CRYPTSETUP} AS pkg-cryptsetup-arm64

FROM --platform=amd64 ${PKG_CONTAINERD} AS pkg-containerd-amd64
FROM --platform=arm64 ${PKG_CONTAINERD} AS pkg-containerd-arm64

FROM --platform=amd64 ${PKG_DOSFSTOOLS} AS pkg-dosfstools-amd64
FROM --platform=arm64 ${PKG_DOSFSTOOLS} AS pkg-dosfstools-arm64

FROM --platform=amd64 ${PKG_E2FSPROGS} AS pkg-e2fsprogs-amd64
FROM --platform=arm64 ${PKG_E2FSPROGS} AS pkg-e2fsprogs-arm64

FROM --platform=amd64 ${PKG_SYSTEMD_UDEVD} AS pkg-systemd-udevd-amd64
FROM --platform=arm64 ${PKG_SYSTEMD_UDEVD} AS pkg-systemd-udevd-arm64

FROM --platform=amd64 ${PKG_LIBCAP} AS pkg-libcap-amd64
FROM --platform=arm64 ${PKG_LIBCAP} AS pkg-libcap-arm64

FROM ${PKG_GRUB} AS pkg-grub
FROM --platform=amd64 ${PKG_GRUB} AS pkg-grub-amd64
FROM --platform=arm64 ${PKG_GRUB} AS pkg-grub-arm64

FROM ${PKG_SD_BOOT} AS pkg-sd-boot
FROM --platform=amd64 ${PKG_SD_BOOT} AS pkg-sd-boot-amd64
FROM --platform=arm64 ${PKG_SD_BOOT} AS pkg-sd-boot-arm64

FROM --platform=amd64 ${PKG_IPTABLES} AS pkg-iptables-amd64
FROM --platform=arm64 ${PKG_IPTABLES} AS pkg-iptables-arm64

FROM --platform=amd64 ${PKG_IPXE} AS pkg-ipxe-amd64
FROM --platform=arm64 ${PKG_IPXE} AS pkg-ipxe-arm64

FROM --platform=amd64 ${PKG_LIBINIH} AS pkg-libinih-amd64
FROM --platform=arm64 ${PKG_LIBINIH} AS pkg-libinih-arm64

FROM --platform=amd64 ${PKG_LIBJSON_C} AS pkg-libjson-c-amd64
FROM --platform=arm64 ${PKG_LIBJSON_C} AS pkg-libjson-c-arm64

FROM --platform=amd64 ${PKG_LIBMNL} AS pkg-libmnl-amd64
FROM --platform=arm64 ${PKG_LIBMNL} AS pkg-libmnl-arm64

FROM --platform=amd64 ${PKG_LIBNFTNL} AS pkg-libnftnl-amd64
FROM --platform=arm64 ${PKG_LIBNFTNL} AS pkg-libnftnl-arm64

FROM --platform=amd64 ${PKG_LIBPOPT} AS pkg-libpopt-amd64
FROM --platform=arm64 ${PKG_LIBPOPT} AS pkg-libpopt-arm64

FROM --platform=amd64 ${PKG_LIBURCU} AS pkg-liburcu-amd64
FROM --platform=arm64 ${PKG_LIBURCU} AS pkg-liburcu-arm64

FROM --platform=amd64 ${PKG_LIBSEPOL} AS pkg-libsepol-amd64
FROM --platform=arm64 ${PKG_LIBSEPOL} AS pkg-libsepol-arm64

FROM --platform=amd64 ${PKG_LIBSELINUX} AS pkg-libselinux-amd64
FROM --platform=arm64 ${PKG_LIBSELINUX} AS pkg-libselinux-arm64

FROM --platform=amd64 ${PKG_PCRE2} AS pkg-pcre2-amd64
FROM --platform=arm64 ${PKG_PCRE2} AS pkg-pcre2-arm64

FROM --platform=amd64 ${PKG_OPENSSL} AS pkg-openssl-amd64
FROM --platform=arm64 ${PKG_OPENSSL} AS pkg-openssl-arm64

FROM --platform=amd64 ${PKG_LIBSECCOMP} AS pkg-libseccomp-amd64
FROM --platform=arm64 ${PKG_LIBSECCOMP} AS pkg-libseccomp-arm64

# linux-firmware is not arch-specific
FROM --platform=amd64 ${PKG_LINUX_FIRMWARE} AS pkg-linux-firmware

FROM --platform=amd64 ${PKG_LVM2} AS pkg-lvm2-amd64
FROM --platform=arm64 ${PKG_LVM2} AS pkg-lvm2-arm64

FROM --platform=amd64 ${PKG_LIBAIO} AS pkg-libaio-amd64
FROM --platform=arm64 ${PKG_LIBAIO} AS pkg-libaio-arm64

FROM --platform=amd64 ${PKG_MUSL} AS pkg-musl-amd64
FROM --platform=arm64 ${PKG_MUSL} AS pkg-musl-arm64

FROM --platform=amd64 ${PKG_RUNC} AS pkg-runc-amd64
FROM --platform=arm64 ${PKG_RUNC} AS pkg-runc-arm64

FROM --platform=amd64 ${PKG_XFSPROGS} AS pkg-xfsprogs-amd64
FROM --platform=arm64 ${PKG_XFSPROGS} AS pkg-xfsprogs-arm64

FROM --platform=amd64 ${PKG_UTIL_LINUX} AS pkg-util-linux-amd64
FROM --platform=arm64 ${PKG_UTIL_LINUX} AS pkg-util-linux-arm64

FROM --platform=amd64 ${PKG_KMOD} AS pkg-kmod-amd64
FROM --platform=arm64 ${PKG_KMOD} AS pkg-kmod-arm64

FROM --platform=amd64 ${PKG_CNI} AS pkg-cni-amd64
FROM --platform=arm64 ${PKG_CNI} AS pkg-cni-arm64

FROM --platform=amd64 ${PKG_FLANNEL_CNI} AS pkg-flannel-cni-amd64
FROM --platform=arm64 ${PKG_FLANNEL_CNI} AS pkg-flannel-cni-arm64

FROM ${PKG_KERNEL} AS pkg-kernel
FROM --platform=amd64 ${PKG_KERNEL} AS pkg-kernel-amd64
FROM --platform=arm64 ${PKG_KERNEL} AS pkg-kernel-arm64

FROM --platform=amd64 ${TOOLS} AS tools-amd64
FROM --platform=arm64 ${TOOLS} AS tools-arm64

FROM scratch AS pkg-debug-tools-scratch-amd64
FROM scratch AS pkg-debug-tools-scratch-arm64

FROM scratch AS pkg-debug-tools-bash-minimal-amd64
COPY --from=tools-amd64 /toolchain/bin/bash /toolchain/bin/bash
COPY --from=tools-amd64 /toolchain/lib/ld-musl-x86_64.so.1 /toolchain/toolchain/lib/ld-musl-x86_64.so.1
COPY --from=tools-amd64 /toolchain/bin/cat /toolchain/bin/cat
COPY --from=tools-amd64 /toolchain/bin/ls /toolchain/bin/ls
COPY --from=tools-amd64 /toolchain/bin/tee /toolchain/bin/tee

FROM scratch AS pkg-debug-tools-bash-minimal-arm64
COPY --from=tools-arm64 /toolchain/bin/bash /toolchain/bin/bash
COPY --from=tools-arm64 /toolchain/lib/ld-musl-aarch64.so.1 /toolchain/toolchain/lib/ld-musl-aarch64.so.1
COPY --from=tools-arm64 /toolchain/bin/cat /toolchain/bin/cat
COPY --from=tools-arm64 /toolchain/bin/ls /toolchain/bin/ls
COPY --from=tools-arm64 /toolchain/bin/tee /toolchain/bin/tee

FROM pkg-debug-tools-${DEBUG_TOOLS_SOURCE}-amd64 AS pkg-debug-tools-amd64
FROM pkg-debug-tools-${DEBUG_TOOLS_SOURCE}-arm64 AS pkg-debug-tools-arm64

# Strip CNI package.

FROM scratch AS pkg-cni-stripped-amd64
COPY --from=pkg-cni-amd64 /opt/cni/bin/bridge /opt/cni/bin/bridge
COPY --from=pkg-cni-amd64 /opt/cni/bin/firewall /opt/cni/bin/firewall
COPY --from=pkg-cni-amd64 /opt/cni/bin/host-local /opt/cni/bin/host-local
COPY --from=pkg-cni-amd64 /opt/cni/bin/loopback /opt/cni/bin/loopback
COPY --from=pkg-cni-amd64 /opt/cni/bin/portmap /opt/cni/bin/portmap

FROM scratch AS pkg-cni-stripped-arm64
COPY --from=pkg-cni-arm64 /opt/cni/bin/bridge /opt/cni/bin/bridge
COPY --from=pkg-cni-arm64 /opt/cni/bin/firewall /opt/cni/bin/firewall
COPY --from=pkg-cni-arm64 /opt/cni/bin/host-local /opt/cni/bin/host-local
COPY --from=pkg-cni-arm64 /opt/cni/bin/loopback /opt/cni/bin/loopback
COPY --from=pkg-cni-arm64 /opt/cni/bin/portmap /opt/cni/bin/portmap

# Resolve package images using ${EXTRAS} to be used later in COPY --from=.

FROM ${PKG_TALOSCTL_CNI_BUNDLE_INSTALL} AS extras-talosctl-cni-bundle-install

# The tools target provides base toolchain for the build.

FROM --platform=${BUILDPLATFORM} $TOOLS AS tools
ENV PATH=/toolchain/bin:/toolchain/go/bin
ENV LD_LIBRARY_PATH=/toolchain/lib
ENV GOTOOLCHAIN=local
RUN ["/toolchain/bin/mkdir", "/bin", "/tmp"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/bin/bash", "/bin/sh"]
RUN ["/toolchain/bin/ln", "-svf", "/toolchain/etc/ssl", "/etc/ssl"]
ARG GOLANGCILINT_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCILINT_VERSION} \
	&& mv /go/bin/golangci-lint /toolchain/go/bin/golangci-lint
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
RUN --mount=type=cache,target=/.cache go install github.com/dmarkham/enumer@${ENUMER_VERSION} \
    && mv /go/bin/enumer /toolchain/go/bin/enumer
ARG DEEPCOPY_GEN_VERSION
RUN --mount=type=cache,target=/.cache go install k8s.io/code-generator/cmd/deepcopy-gen@${DEEPCOPY_GEN_VERSION} \
    && mv /go/bin/deepcopy-gen /toolchain/go/bin/deepcopy-gen
ARG VTPROTOBUF_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@${VTPROTOBUF_VERSION} \
    && mv /go/bin/protoc-gen-go-vtproto /toolchain/go/bin/protoc-gen-go-vtproto
ARG IMPORTVET_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/siderolabs/importvet/cmd/importvet@${IMPORTVET_VERSION} \
    && mv /go/bin/importvet /toolchain/go/bin/importvet
RUN --mount=type=cache,target=/.cache go install golang.org/x/vuln/cmd/govulncheck@latest \
    && mv /go/bin/govulncheck /toolchain/go/bin/govulncheck
ARG PROTOTOOL_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/uber/prototool/cmd/prototool@${PROTOTOOL_VERSION} \
    && mv /go/bin/prototool /toolchain/go/bin/prototool
ARG PROTOC_GEN_DOC_VERSION
RUN --mount=type=cache,target=/.cache go install github.com/pseudomuto/protoc-gen-doc/cmd/protoc-gen-doc@${PROTOC_GEN_DOC_VERSION} \
    && mv /go/bin/protoc-gen-doc /toolchain/go/bin/protoc-gen-doc
COPY ./hack/docgen /go/src/github.com/siderolabs/talos-hack-docgen
RUN --mount=type=cache,target=/.cache cd /go/src/github.com/siderolabs/talos-hack-docgen \
    && go build -o docgen . \
    && mv docgen /toolchain/go/bin/
COPY ./hack/gotagsrewrite /go/src/github.com/siderolabs/gotagsrewrite
RUN --mount=type=cache,target=/.cache cd /go/src/github.com/siderolabs/gotagsrewrite \
    && go build -o gotagsrewrite . \
    && mv gotagsrewrite /toolchain/go/bin/
COPY ./hack/structprotogen /go/src/github.com/siderolabs/structprotogen
RUN --mount=type=cache,target=/.cache cd /go/src/github.com/siderolabs/structprotogen \
    && go build -o structprotogen . \
    && mv structprotogen /toolchain/go/bin/

# The build target creates a container that will be used to build Talos source
# code.

FROM --platform=${BUILDPLATFORM} tools AS build
SHELL ["/toolchain/bin/bash", "-c"]
ENV PATH=/toolchain/bin:/toolchain/go/bin
ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org
ARG CGO_ENABLED
ENV CGO_ENABLED=${CGO_ENABLED}
ENV GOCACHE=/.cache/go-build
ENV GOMODCACHE=/.cache/mod
ENV PROTOTOOL_CACHE_PATH=/.cache/prototool
ARG SOURCE_DATE_EPOCH
ENV SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}
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
RUN mkdir -p _out && \
    echo PKGS=${PKGS} >> _out/talos-metadata && \
    echo TAG=${TAG} >> _out/talos-metadata && \
    echo EXTRAS=${EXTRAS} >> _out/talos-metadata

FROM scratch AS embed-abbrev
COPY --from=embed-abbrev-generate /src/pkg/machinery/gendata/data /pkg/machinery/gendata/data
COPY --from=embed-abbrev-generate /src/_out/talos-metadata /_out/talos-metadata

FROM ${EMBED_TARGET} AS embed-target

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

# run docgen for machinery config
FROM build-go AS go-generate
COPY ./pkg ./pkg
COPY ./hack/boilerplate.txt ./hack/boilerplate.txt
COPY --from=embed-target / ./
RUN --mount=type=cache,target=/.cache go generate ./pkg/...
RUN goimports -w -local github.com/siderolabs/talos ./pkg/
RUN gofumpt -w ./pkg/
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache go generate ./...
RUN gotagsrewrite .
RUN goimports -w -local github.com/siderolabs/talos ./
RUN gofumpt -w ./

FROM go-generate AS gen-proto-go
WORKDIR /src/
RUN --mount=type=cache,target=/.cache structprotogen github.com/siderolabs/talos/pkg/machinery/... /api/resource/definitions/

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
COPY ./api/resource/config/config.proto /api/resource/config/config.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size resource/config/config.proto
COPY ./api/resource/network/device_config.proto /api/resource/network/device_config.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size resource/network/device_config.proto
COPY ./api/inspect/inspect.proto /api/inspect/inspect.proto
RUN protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size inspect/inspect.proto
COPY --from=gen-proto-go /api/resource/definitions/ /api/resource/definitions/
RUN find /api/resource/definitions/ -type f -name "*.proto" | xargs -I {} /bin/sh -c 'protoc -I/api -I/api/vendor/ --go_out=paths=source_relative:/api --go-grpc_out=paths=source_relative:/api --go-vtproto_out=paths=source_relative:/api --go-vtproto_opt=features=marshal+unmarshal+size {} && mkdir -p /api/resource/definitions_go/$(basename {} .proto) && mv /api/resource/definitions/$(basename {} .proto)/*.go /api/resource/definitions_go/$(basename {} .proto)'
# Goimports and gofumpt generated files to adjust import order
RUN goimports -w -local github.com/siderolabs/talos /api/
RUN gofumpt -w /api/

FROM tools AS selinux
COPY ./internal/pkg/selinux/policy/* /selinux/
RUN mkdir /policy; secilc -o /policy/policy.33 -f /policy/file_contexts -c 33 /selinux/**/*.cil -vvvvv -O

FROM scratch AS selinux-generate
COPY --from=selinux /policy /policy

FROM scratch AS ipxe-generate
COPY --from=pkg-ipxe-amd64 /usr/libexec/snp.efi /amd64/snp.efi
COPY --from=pkg-ipxe-arm64 /usr/libexec/snp.efi /arm64/snp.efi

FROM scratch AS microsoft-secureboot-database
ARG MICROSOFT_SECUREBOOT_RELEASE
ADD https://github.com/microsoft/secureboot_objects.git#${MICROSOFT_SECUREBOOT_RELEASE}:PreSignedObjects /

FROM scratch AS microsoft-key-keys
COPY --from=microsoft-secureboot-database /KEK/Certificates/*.der /kek/

FROM scratch AS microsoft-db-keys
COPY --from=microsoft-secureboot-database /DB/Certificates/MicCor*.der /db/
COPY --from=microsoft-secureboot-database /DB/Certificates/microsoft*.der /db/

FROM --platform=${BUILDPLATFORM} scratch AS generate
COPY --from=proto-format-build /src/api /api/
COPY --from=generate-build /api/common/*.pb.go /pkg/machinery/api/common/
COPY --from=generate-build /api/resource/definitions/ /api/resource/definitions/
COPY --from=generate-build /api/resource/definitions_go/ /pkg/machinery/api/resource/definitions/
COPY --from=generate-build /api/security/*.pb.go /pkg/machinery/api/security/
COPY --from=generate-build /api/machine/*.pb.go /pkg/machinery/api/machine/
COPY --from=generate-build /api/time/*.pb.go /pkg/machinery/api/time/
COPY --from=generate-build /api/cluster/*.pb.go /pkg/machinery/api/cluster/
COPY --from=generate-build /api/storage/*.pb.go /pkg/machinery/api/storage/
COPY --from=generate-build /api/resource/*.pb.go /pkg/machinery/api/resource/
COPY --from=generate-build /api/resource/config/*.pb.go /pkg/machinery/api/resource/config/
COPY --from=generate-build /api/resource/network/*.pb.go /pkg/machinery/api/resource/network/
COPY --from=generate-build /api/inspect/*.pb.go /pkg/machinery/api/inspect/
COPY --from=go-generate /src/pkg/flannel/ /pkg/flannel/
COPY --from=go-generate /src/pkg/imager/profile/ /pkg/imager/profile/
COPY --from=go-generate /src/pkg/machinery/resources/ /pkg/machinery/resources/
COPY --from=go-generate /src/pkg/machinery/config/schemas/ /pkg/machinery/config/schemas/
COPY --from=go-generate /src/pkg/machinery/config/types/ /pkg/machinery/config/types/
COPY --from=go-generate /src/pkg/machinery/nethelpers/ /pkg/machinery/nethelpers/
COPY --from=go-generate /src/pkg/machinery/extensions/ /pkg/machinery/extensions/
COPY --from=go-generate /src/pkg/machinery/version/os-release /pkg/machinery/version/os-release
COPY --from=ipxe-generate / /pkg/provision/providers/vm/internal/ipxe/data/ipxe/
COPY --from=selinux-generate / /internal/pkg/selinux/
COPY --from=embed-abbrev / /
COPY --from=pkg-ca-certificates /etc/ssl/certs/ca-certificates /internal/app/machined/pkg/controllers/secrets/data/
COPY --from=microsoft-key-keys / /internal/pkg/secureboot/database/certs/
COPY --from=microsoft-db-keys / /internal/pkg/secureboot/database/certs/

# The base target provides a container that can be used to build all Talos
# assets.

FROM build-go AS base
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY --from=generate /pkg/flannel/ ./pkg/flannel/
COPY --from=generate /pkg/imager/ ./pkg/imager/
COPY --from=generate /pkg/machinery/ ./pkg/machinery/
COPY --from=generate /internal/app/machined/pkg/controllers/secrets/data/ ./internal/app/machined/pkg/controllers/secrets/data/
COPY --from=generate /internal/pkg/secureboot/database/certs/ ./internal/pkg/secureboot/database/certs/
COPY --from=generate /internal/pkg/selinux/ ./internal/pkg/selinux/
COPY --from=embed / ./
RUN --mount=type=cache,target=/.cache go list all >/dev/null
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache go list all >/dev/null
WORKDIR /src

# The vulncheck target runs the vulnerability check tool.

FROM base AS lint-vulncheck
RUN --mount=type=cache,target=/.cache govulncheck ./...

# The init target builds the init binary.

FROM base AS init-build-amd64
WORKDIR /src/internal/app/init
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=v1 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /init
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
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-amd64
RUN chmod +x /talosctl-linux-amd64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-linux-amd64

FROM base AS talosctl-linux-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-arm64
RUN chmod +x /talosctl-linux-arm64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-linux-arm64

FROM base AS talosctl-linux-armv7-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=arm GOARM=7 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-armv7
RUN chmod +x /talosctl-linux-armv7
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-linux-armv7

FROM base AS talosctl-darwin-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=darwin GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-darwin-amd64
RUN chmod +x /talosctl-darwin-amd64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-darwin-amd64

FROM base AS talosctl-darwin-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=darwin GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-darwin-arm64
RUN chmod +x /talosctl-darwin-arm64
RUN touch --date="@${SOURCE_DATE_EPOCH}" talosctl-darwin-arm64

FROM base AS talosctl-windows-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=windows GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-windows-amd64.exe
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-windows-amd64.exe

FROM base AS talosctl-windows-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=windows GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-windows-arm64.exe
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-windows-arm64.exe

FROM base AS talosctl-freebsd-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=freebsd GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-freebsd-amd64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-freebsd-amd64

FROM base AS talosctl-freebsd-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=freebsd GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-freebsd-arm64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-freebsd-arm64

FROM scratch AS talosctl-linux-amd64
COPY --from=talosctl-linux-amd64-build /talosctl-linux-amd64 /talosctl-linux-amd64

FROM scratch AS talosctl-linux-arm64
COPY --from=talosctl-linux-arm64-build /talosctl-linux-arm64 /talosctl-linux-arm64

FROM scratch AS talosctl-linux-armv7
COPY --from=talosctl-linux-armv7-build /talosctl-linux-armv7 /talosctl-linux-armv7

FROM scratch AS talosctl-darwin-amd64
COPY --from=talosctl-darwin-amd64-build /talosctl-darwin-amd64 /talosctl-darwin-amd64

FROM scratch AS talosctl-darwin-arm64
COPY --from=talosctl-darwin-arm64-build /talosctl-darwin-arm64 /talosctl-darwin-arm64

FROM scratch AS talosctl-freebsd-amd64
COPY --from=talosctl-freebsd-amd64-build /talosctl-freebsd-amd64 /talosctl-freebsd-amd64

FROM scratch AS talosctl-freebsd-arm64
COPY --from=talosctl-freebsd-arm64-build /talosctl-freebsd-arm64 /talosctl-freebsd-arm64

FROM scratch AS talosctl-windows-amd64
COPY --from=talosctl-windows-amd64-build /talosctl-windows-amd64.exe /talosctl-windows-amd64.exe

FROM scratch AS talosctl-windows-arm64
COPY --from=talosctl-windows-arm64-build /talosctl-windows-arm64.exe /talosctl-windows-arm64.exe

FROM --platform=${BUILDPLATFORM} talosctl-${TARGETOS}-${TARGETARCH} AS talosctl-targetarch

FROM scratch AS talosctl-all
COPY --from=talosctl-linux-amd64 / /
COPY --from=talosctl-linux-arm64 / /
COPY --from=talosctl-linux-armv7 / /
COPY --from=talosctl-darwin-amd64 / /
COPY --from=talosctl-darwin-arm64 / /
COPY --from=talosctl-freebsd-amd64 / /
COPY --from=talosctl-freebsd-arm64 / /
COPY --from=talosctl-windows-amd64 / /
COPY --from=talosctl-windows-arm64 / /

FROM scratch AS talosctl
ARG TARGETARCH
COPY --from=talosctl-all /talosctl-linux-${TARGETARCH} /talosctl
ARG TAG
ENV VERSION=${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
LABEL org.opencontainers.image.source=https://github.com/siderolabs/talos
ENTRYPOINT ["/talosctl"]

# The kernel target is the linux kernel.
FROM scratch AS kernel
ARG TARGETARCH
COPY --from=pkg-kernel /boot/vmlinuz /vmlinuz-${TARGETARCH}

# The sd-boot target is the systemd-boot asset.
FROM scratch AS sd-boot
ARG TARGETARCH
COPY --from=pkg-sd-boot /*.efi /sd-boot-${TARGETARCH}.efi

# The sd-stub target is the systemd-stub asset.
FROM scratch AS sd-stub
ARG TARGETARCH
COPY --from=pkg-sd-boot /*.efi.stub /sd-stub-${TARGETARCH}.efi

FROM tools AS depmod-amd64
WORKDIR /staging
COPY hack/modules-amd64.txt .
COPY --from=pkg-kernel-amd64 /lib/modules lib/modules
RUN <<EOF
set -euo pipefail

KERNEL_VERSION=$(ls lib/modules)

xargs -a modules-amd64.txt -I {} install -D lib/modules/${KERNEL_VERSION}/{} /build/lib/modules/${KERNEL_VERSION}/{}

depmod -b /build ${KERNEL_VERSION}
EOF

FROM scratch AS modules-amd64
COPY --from=depmod-amd64 /build/lib/modules /lib/modules

FROM tools AS depmod-arm64
WORKDIR /staging
COPY hack/modules-arm64.txt .
COPY --from=pkg-kernel-arm64 /lib/modules lib/modules
RUN <<EOF
set -euo pipefail

KERNEL_VERSION=$(ls lib/modules)

xargs -a modules-arm64.txt -I {} install -D lib/modules/${KERNEL_VERSION}/{} /build/lib/modules/${KERNEL_VERSION}/{}

depmod -b /build ${KERNEL_VERSION}
EOF

FROM scratch AS modules-arm64
COPY --from=depmod-arm64 /build/lib/modules /lib/modules

# The rootfs target provides the Talos rootfs.
FROM build AS rootfs-base-amd64
COPY --link --from=pkg-fhs / /rootfs
COPY --link --from=pkg-apparmor-amd64 / /rootfs
COPY --link --from=pkg-cni-stripped-amd64 / /rootfs
COPY --link --from=pkg-flannel-cni-amd64 / /rootfs
COPY --link --from=pkg-cryptsetup-amd64 / /rootfs
COPY --link --from=pkg-containerd-amd64 / /rootfs
COPY --link --from=pkg-dosfstools-amd64 / /rootfs
COPY --link --from=pkg-e2fsprogs-amd64 / /rootfs
COPY --link --from=pkg-systemd-udevd-amd64 / /rootfs
COPY --link --from=pkg-libcap-amd64 / /rootfs
COPY --link --from=pkg-iptables-amd64 / /rootfs
COPY --link --from=pkg-libinih-amd64 / /rootfs
COPY --link --from=pkg-libjson-c-amd64 / /rootfs
COPY --link --from=pkg-libmnl-amd64 / /rootfs
COPY --link --from=pkg-libnftnl-amd64 / /rootfs
COPY --link --from=pkg-libpopt-amd64 / /rootfs
COPY --link --from=pkg-liburcu-amd64 / /rootfs
COPY --link --from=pkg-libsepol-amd64 / /rootfs
COPY --link --from=pkg-libselinux-amd64 / /rootfs
COPY --link --from=pkg-pcre2-amd64 / /rootfs
COPY --link --from=pkg-openssl-amd64 / /rootfs
COPY --link --from=pkg-libseccomp-amd64 / /rootfs
COPY --link --from=pkg-lvm2-amd64 / /rootfs
COPY --link --from=pkg-libaio-amd64 / /rootfs
COPY --link --from=pkg-musl-amd64 / /rootfs
COPY --link --from=pkg-runc-amd64 / /rootfs
COPY --link --from=pkg-xfsprogs-amd64 / /rootfs
COPY --link --from=pkg-util-linux-amd64 /lib/libblkid.* /rootfs/lib/
COPY --link --from=pkg-util-linux-amd64 /lib/libuuid.* /rootfs/lib/
COPY --link --from=pkg-util-linux-amd64 /lib/libmount.* /rootfs/lib/
COPY --link --from=pkg-kmod-amd64 /usr/lib/libkmod.* /rootfs/lib/
COPY --link --from=pkg-kmod-amd64 /usr/bin/kmod /rootfs/sbin/modprobe
COPY --link --from=modules-amd64 /lib/modules /rootfs/lib/modules
COPY --link --from=machined-build-amd64 /machined /rootfs/sbin/init

# this is a no-op as it copies from a scratch image when WITH_DEBUG_SHELL is not set
COPY --link --from=pkg-debug-tools-amd64 * /rootfs/

RUN <<END
    # the orderly_poweroff call by the kernel will call '/sbin/poweroff'
    ln /rootfs/sbin/init /rootfs/sbin/poweroff
    chmod +x /rootfs/sbin/poweroff
    # some extensions like qemu-guest agent will call '/sbin/shutdown'
    ln /rootfs/sbin/init /rootfs/sbin/shutdown
    chmod +x /rootfs/sbin/shutdown
    ln /rootfs/sbin/init /rootfs/sbin/dashboard
    chmod +x /rootfs/sbin/dashboard
END
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
COPY ./hack/cleanup.sh /toolchain/bin/cleanup.sh
RUN <<END
    cleanup.sh /rootfs
    mkdir -pv /rootfs/{boot/EFI,etc/cri/conf.d/hosts,lib/firmware,usr/etc,usr/local/share,usr/share/zoneinfo/Etc,mnt,system,opt,.extra}
    mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni/net.d,etc/ssl/certs,usr/libexec/kubernetes,/usr/local/lib/kubelet/credentialproviders,etc/selinux/targeted/contexts/files}
    mkdir -pv /rootfs/opt/{containerd/bin,containerd/lib}
END
COPY --chmod=0644 hack/zoneinfo/Etc/UTC /rootfs/usr/share/zoneinfo/Etc/UTC
COPY --chmod=0644 hack/nfsmount.conf /rootfs/etc/nfsmount.conf
COPY --chmod=0644 hack/containerd.toml /rootfs/etc/containerd/config.toml
COPY --chmod=0644 hack/cri-containerd.toml /rootfs/etc/cri/containerd.toml
COPY --chmod=0644 hack/cri-plugin.part /rootfs/etc/cri/conf.d/00-base.part
COPY --chmod=0644 hack/udevd/99-default.link /rootfs/usr/lib/systemd/network/
COPY --chmod=0644 hack/udevd/90-selinux.rules /rootfs/usr/lib/udev/rules.d/
COPY --chmod=0644 hack/lvm.conf /rootfs/etc/lvm/lvm.conf
COPY --chmod=0644 --from=base /src/pkg/machinery/version/os-release /rootfs/etc/os-release
RUN <<END
    ln -s /usr/share/zoneinfo/Etc/UTC /rootfs/etc/localtime
    touch /rootfs/etc/{extensions.yaml,resolv.conf,hosts,machine-id,cri/conf.d/cri.toml,cri/conf.d/01-registries.part,cri/conf.d/20-customization.part,cri/conf.d/base-spec.json,ssl/certs/ca-certificates,selinux/targeted/contexts/files/file_contexts}
    ln -s ca-certificates /rootfs/etc/ssl/certs/ca-certificates.crt
    ln -s /etc/ssl /rootfs/etc/pki
    ln -s /etc/ssl /rootfs/usr/share/ca-certificates
    ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
    ln -s /etc/ssl /rootfs/etc/ca-certificates
END

FROM build AS rootfs-base-arm64
COPY --link --from=pkg-fhs / /rootfs
COPY --link --from=pkg-apparmor-arm64 / /rootfs
COPY --link --from=pkg-cni-stripped-arm64 / /rootfs
COPY --link --from=pkg-flannel-cni-arm64 / /rootfs
COPY --link --from=pkg-cryptsetup-arm64 / /rootfs
COPY --link --from=pkg-containerd-arm64 / /rootfs
COPY --link --from=pkg-dosfstools-arm64 / /rootfs
COPY --link --from=pkg-e2fsprogs-arm64 / /rootfs
COPY --link --from=pkg-systemd-udevd-arm64 / /rootfs
COPY --link --from=pkg-libcap-arm64 / /rootfs
COPY --link --from=pkg-iptables-arm64 / /rootfs
COPY --link --from=pkg-libinih-arm64 / /rootfs
COPY --link --from=pkg-libjson-c-arm64 / /rootfs
COPY --link --from=pkg-libmnl-arm64 / /rootfs
COPY --link --from=pkg-libnftnl-arm64 / /rootfs
COPY --link --from=pkg-libpopt-arm64 / /rootfs
COPY --link --from=pkg-liburcu-arm64 / /rootfs
COPY --link --from=pkg-libsepol-arm64 / /rootfs
COPY --link --from=pkg-libselinux-arm64 / /rootfs
COPY --link --from=pkg-pcre2-arm64 / /rootfs
COPY --link --from=pkg-openssl-arm64 / /rootfs
COPY --link --from=pkg-libseccomp-arm64 / /rootfs
COPY --link --from=pkg-lvm2-arm64 / /rootfs
COPY --link --from=pkg-libaio-arm64 / /rootfs
COPY --link --from=pkg-musl-arm64 / /rootfs
COPY --link --from=pkg-runc-arm64 / /rootfs
COPY --link --from=pkg-xfsprogs-arm64 / /rootfs
COPY --link --from=pkg-util-linux-arm64 /lib/libblkid.* /rootfs/lib/
COPY --link --from=pkg-util-linux-arm64 /lib/libuuid.* /rootfs/lib/
COPY --link --from=pkg-util-linux-arm64 /lib/libmount.* /rootfs/lib/
COPY --link --from=pkg-kmod-arm64 /usr/lib/libkmod.* /rootfs/lib/
COPY --link --from=pkg-kmod-arm64 /usr/bin/kmod /rootfs/sbin/modprobe
COPY --link --from=modules-arm64 /lib/modules /rootfs/lib/modules
COPY --link --from=machined-build-arm64 /machined /rootfs/sbin/init

# this is a no-op as it copies from a scratch image when WITH_DEBUG_SHELL is not set
COPY --link --from=pkg-debug-tools-arm64 * /rootfs/

RUN <<END
    # the orderly_poweroff call by the kernel will call '/sbin/poweroff'
    ln /rootfs/sbin/init /rootfs/sbin/poweroff
    chmod +x /rootfs/sbin/poweroff
    # some extensions like qemu-guest agent will call '/sbin/shutdown'
    ln /rootfs/sbin/init /rootfs/sbin/shutdown
    chmod +x /rootfs/sbin/shutdown
    ln /rootfs/sbin/init /rootfs/sbin/dashboard
    chmod +x /rootfs/sbin/dashboard
END
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
COPY ./hack/cleanup.sh /toolchain/bin/cleanup.sh
RUN <<END
    cleanup.sh /rootfs
    mkdir -pv /rootfs/{boot/EFI,etc/cri/conf.d/hosts,lib/firmware,usr/etc,usr/local/share,usr/share/zoneinfo/Etc,mnt,system,opt,.extra}
    mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni/net.d,etc/ssl/certs,usr/libexec/kubernetes,/usr/local/lib/kubelet/credentialproviders,etc/selinux/targeted/contexts/files}
    mkdir -pv /rootfs/opt/{containerd/bin,containerd/lib}
END
COPY --chmod=0644 hack/zoneinfo/Etc/UTC /rootfs/usr/share/zoneinfo/Etc/UTC
COPY --chmod=0644 hack/nfsmount.conf /rootfs/etc/nfsmount.conf
COPY --chmod=0644 hack/containerd.toml /rootfs/etc/containerd/config.toml
COPY --chmod=0644 hack/cri-containerd.toml /rootfs/etc/cri/containerd.toml
COPY --chmod=0644 hack/cri-plugin.part /rootfs/etc/cri/conf.d/00-base.part
COPY --chmod=0644 hack/udevd/99-default.link /rootfs/usr/lib/systemd/network/
COPY --chmod=0644 hack/udevd/90-selinux.rules /rootfs/usr/lib/udev/rules.d/
COPY --chmod=0644 hack/lvm.conf /rootfs/etc/lvm/lvm.conf
COPY --chmod=0644 --from=base /src/pkg/machinery/version/os-release /rootfs/etc/os-release
RUN <<END
    ln -s /usr/share/zoneinfo/Etc/UTC /rootfs/etc/localtime
    touch /rootfs/etc/{extensions.yaml,resolv.conf,hosts,machine-id,cri/conf.d/cri.toml,cri/conf.d/01-registries.part,cri/conf.d/20-customization.part,cri/conf.d/base-spec.json,ssl/certs/ca-certificates,selinux/targeted/contexts/files/file_contexts}
    ln -s /etc/ssl /rootfs/etc/pki
    ln -s ca-certificates /rootfs/etc/ssl/certs/ca-certificates.crt
    ln -s /etc/ssl /rootfs/usr/share/ca-certificates
    ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
    ln -s /etc/ssl /rootfs/etc/ca-certificates
END

FROM rootfs-base-${TARGETARCH} AS rootfs-base
RUN echo "true" > /rootfs/usr/etc/in-container
RUN rm -rf /rootfs/lib/modules/*
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"

FROM rootfs-base-arm64 AS rootfs-squashfs-arm64
ARG ZSTD_COMPRESSION_LEVEL
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
COPY --from=selinux-generate /policy/file_contexts /file_contexts
COPY ./hack/labeled-squashfs.sh /
ENV SHELL=/toolchain/bin/bash
RUN fakeroot /labeled-squashfs.sh /rootfs /rootfs.sqsh /file_contexts ${ZSTD_COMPRESSION_LEVEL}

FROM rootfs-base-amd64 AS rootfs-squashfs-amd64
ARG ZSTD_COMPRESSION_LEVEL
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
COPY --from=selinux-generate /policy/file_contexts /file_contexts
COPY ./hack/labeled-squashfs.sh /
ENV SHELL=/toolchain/bin/bash
RUN fakeroot /labeled-squashfs.sh /rootfs /rootfs.sqsh /file_contexts ${ZSTD_COMPRESSION_LEVEL}

FROM scratch AS squashfs-arm64
COPY --from=rootfs-squashfs-arm64 /rootfs.sqsh /

FROM scratch AS squashfs-amd64
COPY --from=rootfs-squashfs-amd64 /rootfs.sqsh /

FROM scratch AS rootfs
COPY --from=rootfs-base /rootfs /

# The initramfs target provides the Talos initramfs image.

FROM build AS initramfs-archive-arm64
WORKDIR /initramfs
ARG ZSTD_COMPRESSION_LEVEL
COPY --from=squashfs-arm64 /rootfs.sqsh .
COPY --from=init-build-arm64 /init .
RUN find . -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
RUN set -o pipefail \
    && find . 2>/dev/null \
    | LC_ALL=c sort \
    | cpio --reproducible -H newc -o \
    | zstd -c -T0 -${ZSTD_COMPRESSION_LEVEL} \
    > /initramfs.xz

FROM build AS initramfs-archive-amd64
WORKDIR /initramfs
ARG ZSTD_COMPRESSION_LEVEL
COPY --from=squashfs-amd64 /rootfs.sqsh .
COPY --from=init-build-amd64 /init .
RUN find . -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
RUN set -o pipefail \
    && find . 2>/dev/null \
    | LC_ALL=c sort \
    | cpio --reproducible -H newc -o \
    | zstd -c -T0 -${ZSTD_COMPRESSION_LEVEL} \
    > /initramfs.xz

FROM initramfs-archive-${TARGETARCH} AS initramfs-archive

FROM scratch AS initramfs
ARG TARGETARCH
COPY --from=initramfs-archive /initramfs.xz /initramfs-${TARGETARCH}.xz

# The talos target generates a docker image that can be used to run Talos
# in containers.

FROM scratch AS talos
COPY --from=rootfs / /
LABEL org.opencontainers.image.source=https://github.com/siderolabs/talos
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

FROM alpine:3.21.0 AS unicode-pf2
RUN apk add --no-cache --update --no-scripts grub

FROM scratch AS install-artifacts-amd64
COPY --from=pkg-kernel-amd64 /boot/vmlinuz /usr/install/amd64/vmlinuz
COPY --from=initramfs-archive-amd64 /initramfs.xz /usr/install/amd64/initramfs.xz
COPY --from=pkg-sd-boot-amd64 /linuxx64.efi.stub /usr/install/amd64/systemd-stub.efi
COPY --from=pkg-sd-boot-amd64 /systemd-bootx64.efi /usr/install/amd64/systemd-boot.efi

FROM scratch AS install-artifacts-arm64
COPY --from=pkg-kernel-arm64 /boot/vmlinuz /usr/install/arm64/vmlinuz
COPY --from=initramfs-archive-arm64 /initramfs.xz /usr/install/arm64/initramfs.xz
COPY --from=pkg-sd-boot-arm64 /linuxaa64.efi.stub /usr/install/arm64/systemd-stub.efi
COPY --from=pkg-sd-boot-arm64 /systemd-bootaa64.efi /usr/install/arm64/systemd-boot.efi

FROM scratch AS install-artifacts-all
COPY --from=install-artifacts-amd64 / /
COPY --from=install-artifacts-arm64 / /

FROM install-artifacts-${TARGETARCH} AS install-artifacts-targetarch

FROM install-artifacts-${INSTALLER_ARCH} AS install-artifacts

FROM alpine:3.21.0 AS installer-image
ARG SOURCE_DATE_EPOCH
ENV SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}
ARG INSTALLER_PKGS
RUN apk add --no-cache --update --no-scripts \
    ${INSTALLER_PKGS}
ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH}
COPY --from=installer-build /installer /bin/installer
COPY --chmod=0644 hack/extra-modules.conf /etc/modules.d/10-extra-modules.conf
COPY --from=pkg-grub / /
COPY --from=pkg-grub-arm64 /usr/lib/grub /usr/lib/grub
COPY --from=pkg-grub-amd64 /usr/lib/grub /usr/lib/grub
COPY --from=unicode-pf2 /usr/share/grub/unicode.pf2 /usr/share/grub/unicode.pf2
RUN ln /bin/installer /bin/imager
RUN find /bin /etc /lib /usr /sbin | grep -Ev '/etc/hosts|/etc/resolv.conf' \
    | xargs -r touch --date="@${SOURCE_DATE_EPOCH}" --no-dereference

FROM scratch AS installer-image-squashed
COPY --from=installer-image / /
ARG TAG
ENV VERSION=${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
LABEL org.opencontainers.image.source=https://github.com/siderolabs/talos
ENTRYPOINT ["/bin/installer"]

FROM installer-image-squashed AS installer
COPY --from=install-artifacts / /

FROM installer-image-squashed AS imager
COPY --from=install-artifacts / /
ENTRYPOINT ["/bin/imager"]

FROM imager AS iso-amd64-build
ARG SOURCE_DATE_EPOCH
ENV SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}
RUN /bin/installer \
    iso \
    --arch amd64 \
    --output /out

FROM imager AS iso-arm64-build
ARG SOURCE_DATE_EPOCH
ENV SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}
RUN /bin/installer \
    iso \
    --arch arm64 \
    --output /out

FROM scratch AS iso-amd64
COPY --from=iso-amd64-build /out /

FROM scratch AS iso-arm64
COPY --from=iso-arm64-build /out /

FROM --platform=${BUILDPLATFORM} iso-${TARGETARCH} AS iso

# The test target performs tests on the source code.
FROM base AS unit-tests-runner
RUN unlink /etc/ssl
COPY --from=rootfs / /
COPY --from=pkg-ca-certificates / /
ARG TESTPKGS
ENV PLATFORM=container
ARG GO_LDFLAGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache go test -failfast -v \
    -ldflags "${GO_LDFLAGS}" \
    -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -count 1 -p 4 ${TESTPKGS}
FROM scratch AS unit-tests
COPY --from=unit-tests-runner /src/coverage.txt /coverage.txt

# The unit-tests-race target performs tests with race detector.

FROM base AS unit-tests-race
RUN unlink /etc/ssl
COPY --from=rootfs / /
COPY --from=pkg-ca-certificates / /
ARG TESTPKGS
ENV PLATFORM=container
ENV CGO_ENABLED=1
ARG GO_LDFLAGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache go test -v \
    -ldflags "${GO_LDFLAGS}" \
    -race -count 1 -p 4 ${TESTPKGS}

# The integration-test targets builds integration test binary.

FROM base AS integration-test-linux-amd64-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-linux-amd64
COPY --from=integration-test-linux-amd64-build /src/integration.test /integration-test-linux-amd64

FROM base AS integration-test-linux-arm64-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=arm64 go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-linux-arm64
COPY --from=integration-test-linux-arm64-build /src/integration.test /integration-test-linux-arm64

FROM --platform=${BUILDPLATFORM} integration-test-${TARGETOS}-${TARGETARCH} AS integration-test-targetarch

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

# The module-sig-verify targets builds module-sig-verify binary.
FROM build-go AS module-sig-verify-linux-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
WORKDIR /src/module-sig-verify
COPY ./hack/module-sig-verify/go.mod ./hack/module-sig-verify/go.sum ./
RUN --mount=type=cache,target=/.cache go mod download
COPY ./hack/module-sig-verify/main.go .
RUN --mount=type=cache,target=/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build -o module-sig-verify .

FROM scratch AS module-sig-verify-linux
COPY --from=module-sig-verify-linux-build /src/module-sig-verify/module-sig-verify /module-sig-verify-linux-amd64

# The lint target performs linting on the source code.
FROM base AS lint-go
COPY .golangci.yml .
ENV GOGC=50
ENV GOLANGCI_LINT_CACHE=/.cache/lint
RUN golangci-lint config verify --config .golangci.yml
RUN --mount=type=cache,target=/.cache golangci-lint run --config .golangci.yml
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache golangci-lint run --config ../../.golangci.yml
COPY ./hack/cloud-image-uploader /src/hack/cloud-image-uploader
WORKDIR /src/hack/cloud-image-uploader
RUN --mount=type=cache,target=/.cache golangci-lint run --config ../../.golangci.yml
WORKDIR /src
RUN --mount=type=cache,target=/.cache importvet github.com/siderolabs/talos/...

# The protolint target performs linting on protobuf files.

FROM base AS lint-protobuf
WORKDIR /src/api
COPY api .
RUN --mount=type=cache,target=/.cache prototool lint --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include
RUN --mount=type=cache,target=/.cache prototool break check --descriptor-set-path=api.descriptors --protoc-bin-path=/toolchain/bin/protoc --protoc-wkt-path=/toolchain/include

# The markdownlint target performs linting on Markdown files.

FROM oven/bun:1-alpine AS lint-markdown
ARG MARKDOWNLINTCLI_VERSION
ARG TEXTLINT_VERSION
ARG TEXTLINT_FILTER_RULE_COMMENTS_VERSION
ARG TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION
RUN apk add --no-cache findutils
RUN bun i -g markdownlint-cli@${MARKDOWNLINTCLI_VERSION} textlint@${TEXTLINT_VERSION} textlint-filter-rule-comments@${TEXTLINT_FILTER_RULE_COMMENTS_VERSION} textlint-rule-one-sentence-per-line@${TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION}
WORKDIR /src
COPY . .
RUN bun run --bun markdownlint \
    --ignore '**/LICENCE.md' \
    --ignore '**/CHANGELOG.md' \
    --ignore '**/CODE_OF_CONDUCT.md' \
    --ignore '**/node_modules/**' \
    --ignore '**/hack/chglog/**' \
    --ignore 'website/content/*/reference/*' \
    --ignore 'website/themes/**' \
    --disable MD045 MD056 -- \
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
    | xargs -0 bun run --bun textlint

# The docs target generates documentation.

FROM base AS docs-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY --from=talosctl-targetarch /talosctl-${TARGETOS}-${TARGETARCH} /bin/talosctl
RUN env HOME=/home/user TAG=latest /bin/talosctl docs --config /tmp/configuration \
    && env HOME=/home/user TAG=latest /bin/talosctl docs --cli /tmp
COPY ./pkg/machinery/config/schemas/*.schema.json /tmp/schemas/

FROM tools AS proto-docs-build
COPY --from=generate-build /api /protos
COPY ./hack/protoc-gen-doc/markdown.tmpl /tmp/markdown.tmpl
RUN protoc \
    -I/protos \
    -I/protos/common \
    -I/protos/resource/definitions \
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
    /protos/resource/definitions/**/*.proto \
    /protos/inspect/*.proto \
    /protos/machine/*.proto \
    /protos/security/*.proto \
    /protos/storage/*.proto \
    /protos/time/*.proto

FROM scratch AS docs
COPY --from=docs-build /tmp/configuration/ /website/content/v1.10/reference/configuration/
COPY --from=docs-build /tmp/cli.md /website/content/v1.10/reference/
COPY --from=docs-build /tmp/schemas /website/content/v1.10/schemas/
COPY --from=proto-docs-build /tmp/api.md /website/content/v1.10/reference/

# The talosctl-cni-bundle builds the CNI bundle for talosctl.

FROM scratch AS talosctl-cni-bundle
ARG TARGETARCH
COPY --from=extras-talosctl-cni-bundle-install /opt/cni/bin/ /talosctl-cni-bundle-${TARGETARCH}/

# The go-mod-outdated target lists all outdated modules.

FROM base AS go-mod-outdated
RUN --mount=type=cache,target=/.cache go install github.com/psampaz/go-mod-outdated@latest \
    && mv /go/bin/go-mod-outdated /toolchain/go/bin/go-mod-outdated
COPY ./hack/cloud-image-uploader ./hack/cloud-image-uploader
COPY ./hack/docgen ./hack/docgen
COPY ./hack/gotagsrewrite ./hack/gotagsrewrite
COPY ./hack/module-sig-verify ./hack/module-sig-verify
COPY ./hack/structprotogen ./hack/structprotogen
# fail always to get the output back
RUN --mount=type=cache,target=/.cache <<EOF
    for project in pkg/machinery . hack/cloud-image-uploader hack/docgen hack/gotagsrewrite hack/module-sig-verify hack/structprotogen; do
        echo -e "\n>>>> ${project}:" && \
        (cd "${project}" && go list -u -m -json all | go-mod-outdated -update -direct)
    done

    exit 1
EOF
