# syntax = docker/dockerfile-upstream:1.24.0-labs

# Meta args applied to stage base names.

ARG TOOLS=scratch
ARG PKGS=scratch
ARG INSTALLER_ARCH=scratch

ARG PKGS_PREFIX=scratch
ARG TOOLS_PREFIX=scratch

ARG GENERATE_VEX_PREFIX=scratch
ARG GENERATE_VEX=scratch

ARG PKG_APPARMOR=scratch
ARG PKG_BTRFSPROGS=scratch
ARG PKG_CA_CERTIFICATES=scratch
ARG PKG_CNI=scratch
ARG PKG_CONTAINERD=scratch
ARG PKG_CPIO=scratch
ARG PKG_CRYPTSETUP=scratch
ARG PKG_DOSFSTOOLS=scratch
ARG PKG_E2FSPROGS=scratch
ARG PKG_FHS=scratch
ARG PKG_FLANNEL_CNI=scratch
ARG PKG_GLIB=scratch
ARG PKG_GRUB=scratch
ARG PKG_IGZIP=scratch
ARG PKG_IPTABLES=scratch
ARG PKG_IPXE=scratch
ARG PKG_KERNEL=scratch
ARG PKG_KMOD=scratch
ARG PKG_LIBAIO=scratch
ARG PKG_LIBARCHIVE=scratch
ARG PKG_LIBATTR=scratch
ARG PKG_LIBBURN=scratch
ARG PKG_LIBCAP=scratch
ARG PKG_LIBINIH=scratch
ARG PKG_LIBISOBURN=scratch
ARG PKG_LIBISOFS=scratch
ARG PKG_LIBJANSSON=scratch
ARG PKG_LIBJSON_C=scratch
ARG PKG_LIBLZMA=scratch
ARG PKG_LIBMNL=scratch
ARG PKG_LIBNFTNL=scratch
ARG PKG_LIBPOPT=scratch
ARG PKG_LIBSELINUX=scratch
ARG PKG_LIBSEPOL=scratch
ARG PKG_LIBURCU=scratch
ARG PKG_LINUX_FIRMWARE=scratch
ARG PKG_LVM2=scratch
ARG PKG_MTOOLS=scratch
ARG PKG_MUSL=scratch
ARG PKG_NFTABLES=scratch
ARG PKG_OPENSSL=scratch
ARG PKG_OPEN_VMDK=scratch
ARG PKG_PCRE2=scratch
ARG PKG_PIGZ=scratch
ARG PKG_QEMU_TOOLS=scratch
ARG PKG_RUNC=scratch
ARG PKG_SD_BOOT=scratch
ARG PKG_SQUASHFS_TOOLS=scratch
ARG PKG_SYSTEMD_UDEVD=scratch
ARG PKG_TALOSCTL_CNI_BUNDLE=scratch
ARG PKG_TAR=scratch
ARG PKG_UTIL_LINUX=scratch
ARG PKG_XFSPROGS=scratch
ARG PKG_XZ=scratch
ARG PKG_ZLIB=scratch
ARG PKG_ZSTD=scratch

ARG EMBED_TARGET=embed

# Resolve package images using ${PKGS} to be used later in COPY --link --from=.

FROM ${PKG_FHS} AS pkg-fhs
FROM ${PKG_CA_CERTIFICATES} AS pkg-ca-certificates

 # used only for the unit-tests environment
FROM ${PKG_BTRFSPROGS} AS pkg-btrfsprogs

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

FROM --platform=amd64 ${PKG_LIBARCHIVE} AS pkg-libarchive-amd64
FROM --platform=arm64 ${PKG_LIBARCHIVE} AS pkg-libarchive-arm64

FROM --platform=amd64 ${PKG_LIBATTR} AS pkg-libattr-amd64
FROM --platform=arm64 ${PKG_LIBATTR} AS pkg-libattr-arm64

FROM --platform=amd64 ${PKG_LIBINIH} AS pkg-libinih-amd64
FROM --platform=arm64 ${PKG_LIBINIH} AS pkg-libinih-arm64

FROM --platform=amd64 ${PKG_LIBJANSSON} AS pkg-libjansson-amd64
FROM --platform=arm64 ${PKG_LIBJANSSON} AS pkg-libjansson-arm64

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

# linux-firmware is not arch-specific
FROM --platform=amd64 ${PKG_LINUX_FIRMWARE} AS pkg-linux-firmware

FROM --platform=amd64 ${PKG_LVM2} AS pkg-lvm2-amd64
FROM --platform=arm64 ${PKG_LVM2} AS pkg-lvm2-arm64

FROM --platform=amd64 ${PKG_LIBAIO} AS pkg-libaio-amd64
FROM --platform=arm64 ${PKG_LIBAIO} AS pkg-libaio-arm64

FROM --platform=amd64 ${PKG_NFTABLES} AS pkg-nftables-amd64
FROM --platform=arm64 ${PKG_NFTABLES} AS pkg-nftables-arm64

FROM --platform=amd64 ${PKG_MUSL} AS pkg-musl-amd64
FROM --platform=arm64 ${PKG_MUSL} AS pkg-musl-arm64

FROM --platform=amd64 ${PKG_RUNC} AS pkg-runc-amd64
FROM --platform=arm64 ${PKG_RUNC} AS pkg-runc-arm64

FROM --platform=amd64 ${PKG_XFSPROGS} AS pkg-xfsprogs-amd64
FROM --platform=arm64 ${PKG_XFSPROGS} AS pkg-xfsprogs-arm64

FROM ${PKG_UTIL_LINUX} AS pkg-util-linux
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

FROM ${PKG_PIGZ} AS pkg-pigz
FROM --platform=arm64 ${PKG_PIGZ} AS pkg-pigz-arm64

FROM ${PKG_ZLIB} AS pkg-zlib
FROM --platform=amd64 ${PKG_ZLIB} AS pkg-zlib-amd64
FROM --platform=arm64 ${PKG_ZLIB} AS pkg-zlib-arm64

FROM --platform=amd64 ${PKG_IGZIP} AS pkg-igzip-amd64

FROM ${PKG_ZSTD} AS pkg-zstd
FROM --platform=amd64 ${PKG_ZSTD} AS pkg-zstd-amd64
FROM --platform=arm64 ${PKG_ZSTD} AS pkg-zstd-arm64

FROM ${PKG_CPIO} AS pkg-cpio
FROM ${PKG_DOSFSTOOLS} AS pkg-dosfstools
FROM ${PKG_E2FSPROGS} AS pkg-e2fsprogs
FROM ${PKG_GLIB} AS pkg-glib
FROM ${PKG_KMOD} AS pkg-kmod
FROM ${PKG_LIBARCHIVE} AS pkg-libarchive
FROM ${PKG_LIBATTR} AS pkg-libattr
FROM ${PKG_LIBBURN} AS pkg-libburn
FROM ${PKG_LIBINIH} AS pkg-libinih
FROM ${PKG_LIBISOBURN} AS pkg-libisoburn
FROM ${PKG_LIBISOFS} AS pkg-libisofs
FROM ${PKG_LIBLZMA} AS pkg-liblzma
FROM ${PKG_LIBURCU} AS pkg-liburcu
FROM ${PKG_MTOOLS} AS pkg-mtools
FROM ${PKG_MUSL} AS pkg-musl
FROM ${PKG_OPENSSL} AS pkg-openssl
FROM ${PKG_OPEN_VMDK} AS pkg-open-vmdk
FROM ${PKG_PCRE2} AS pkg-pcre2
FROM ${PKG_QEMU_TOOLS} AS pkg-qemu-tools
FROM ${PKG_SQUASHFS_TOOLS} AS pkg-squashfs-tools
FROM ${PKG_TAR} AS pkg-tar
FROM ${PKG_XFSPROGS} AS pkg-xfsprogs
FROM ${PKG_XZ} AS pkg-xz

FROM --platform=amd64 ${TOOLS_PREFIX}:${TOOLS} AS tools-amd64
FROM --platform=arm64 ${TOOLS_PREFIX}:${TOOLS} AS tools-arm64

# Strip CNI package.

FROM scratch AS pkg-cni-stripped-amd64
COPY --link --from=pkg-cni-amd64 /opt/cni/bin/bridge /opt/cni/bin/bridge
COPY --link --from=pkg-cni-amd64 /opt/cni/bin/firewall /opt/cni/bin/firewall
COPY --link --from=pkg-cni-amd64 /opt/cni/bin/host-local /opt/cni/bin/host-local
COPY --link --from=pkg-cni-amd64 /opt/cni/bin/loopback /opt/cni/bin/loopback
COPY --link --from=pkg-cni-amd64 /opt/cni/bin/portmap /opt/cni/bin/portmap
COPY --link --from=pkg-cni-amd64 /usr/share/spdx/cni.spdx.json /usr/share/spdx/cni.spdx.json

FROM scratch AS pkg-cni-stripped-arm64
COPY --link --from=pkg-cni-arm64 /opt/cni/bin/bridge /opt/cni/bin/bridge
COPY --link --from=pkg-cni-arm64 /opt/cni/bin/firewall /opt/cni/bin/firewall
COPY --link --from=pkg-cni-arm64 /opt/cni/bin/host-local /opt/cni/bin/host-local
COPY --link --from=pkg-cni-arm64 /opt/cni/bin/loopback /opt/cni/bin/loopback
COPY --link --from=pkg-cni-arm64 /opt/cni/bin/portmap /opt/cni/bin/portmap
COPY --link --from=pkg-cni-arm64 /usr/share/spdx/cni.spdx.json /usr/share/spdx/cni.spdx.json

FROM ${PKG_TALOSCTL_CNI_BUNDLE} AS pkgs-talosctl-cni-bundle

# The tools target provides base toolchain for the build.

FROM --platform=${BUILDPLATFORM} ${TOOLS_PREFIX}:${TOOLS} AS tools
ENV GOTOOLCHAIN=local
ENV CGO_ENABLED=0
SHELL ["/bin/bash", "-c"]

# The build target creates a container that will be used to build Talos source
# code.

FROM --platform=${BUILDPLATFORM} tools AS build
SHELL ["/bin/bash", "-c"]
ENV GO111MODULE=on
ENV GOPROXY=https://proxy.golang.org
ARG CGO_ENABLED
ENV CGO_ENABLED=${CGO_ENABLED}
ARG GOFIPS140
ENV GOFIPS140=${GOFIPS140}
ENV GOCACHE=/.cache/go-build
ENV GOMODCACHE=/.cache/mod
ARG SOURCE_DATE_EPOCH
ENV SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}
WORKDIR /src

# The build-go target creates a container to build Go code with Go modules downloaded and verified.

FROM build AS build-go
COPY ./go.mod ./go.sum ./go.work ./.custom-gcl.yml ./
COPY ./pkg/machinery/go.mod ./pkg/machinery/go.sum ./pkg/machinery/
COPY ./tools ./tools
WORKDIR /src
RUN --mount=type=cache,target=/.cache,id=talos/.cache go mod download
RUN --mount=type=cache,target=/.cache,id=talos/.cache go mod verify

# The generate target generates code from protobuf service definitions and machinery config.

FROM tools AS embed-generate
WORKDIR /src
ARG NAME
ARG SHA
ARG USERNAME
ARG REGISTRY
ARG FACTORY
ARG TAG
ARG ARTIFACTS
ARG PKGS
ARG TOOLS
RUN mkdir -p pkg/machinery/gendata/data && \
    echo -n ${NAME} > pkg/machinery/gendata/data/name && \
    echo -n ${SHA} > pkg/machinery/gendata/data/sha && \
    echo -n ${USERNAME} > pkg/machinery/gendata/data/username && \
    echo -n ${REGISTRY} > pkg/machinery/gendata/data/registry && \
    echo -n ${FACTORY} > pkg/machinery/gendata/data/factory && \
    echo -n ${PKGS} > pkg/machinery/gendata/data/pkgs && \
    echo -n ${TOOLS} > pkg/machinery/gendata/data/tools && \
    echo -n ${TAG} > pkg/machinery/gendata/data/tag && \
    echo -n ${ARTIFACTS} > pkg/machinery/gendata/data/artifacts

FROM scratch AS embed
COPY --link --from=embed-generate /src/pkg/machinery/gendata/data /pkg/machinery/gendata/data

FROM embed-generate AS embed-abbrev-generate
ARG ABBREV_TAG
RUN echo -n "undefined" > pkg/machinery/gendata/data/sha && \
    echo -n ${ABBREV_TAG} > pkg/machinery/gendata/data/tag
RUN mkdir -p _out && \
    echo PKGS=${PKGS} >> _out/talos-metadata && \
    echo TOOLS=${TOOLS} >> _out/talos-metadata && \
    echo TAG=${TAG} >> _out/talos-metadata

FROM scratch AS embed-abbrev
COPY --link --from=embed-abbrev-generate /src/pkg/machinery/gendata/data /pkg/machinery/gendata/data
COPY --link --from=embed-abbrev-generate /src/_out/talos-metadata /_out/talos-metadata

FROM ${EMBED_TARGET} AS embed-target

# generate API descriptors
FROM build-go AS api-descriptors-build
WORKDIR /src/api
COPY api .
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/bufbuild/buf/cmd/buf format
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/bufbuild/buf/cmd/buf build --exclude-source-info -o lock.binpb

FROM --platform=${BUILDPLATFORM} scratch AS api-descriptors
COPY --link --from=api-descriptors-build /src/api/lock.binpb /api/lock.binpb

# format protobuf service definitions
FROM build-go AS proto-format-build
WORKDIR /src/api
COPY api .
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/bufbuild/buf/cmd/buf format

FROM --platform=${BUILDPLATFORM} scratch AS fmt-protobuf
COPY --link --from=proto-format-build /src/api/ /api/

# run docgen for machinery config
FROM build-go AS go-generate
COPY ./pkg ./pkg
COPY ./hack/boilerplate.txt ./hack/boilerplate.txt
COPY --link --from=embed-target / ./
RUN --mount=type=cache,target=/.cache,id=talos/.cache go generate ./pkg/...
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool golang.org/x/tools/cmd/goimports -w -local github.com/siderolabs/talos ./pkg/
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool mvdan.cc/gofumpt -w ./pkg/
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache,id=talos/.cache go generate ./...
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/siderolabs/talos/tools/gotagsrewrite .
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool golang.org/x/tools/cmd/goimports -w -local github.com/siderolabs/talos ./
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool mvdan.cc/gofumpt -w ./

FROM go-generate AS gen-proto-go
WORKDIR /src/
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/siderolabs/talos/tools/structprotogen github.com/siderolabs/talos/pkg/machinery/... /api/resource/definitions/

# compile protobuf service definitions
FROM build-go AS generate-build
COPY --link --from=proto-format-build /src/api /src/api/
COPY --link --from=gen-proto-go /api/resource/definitions/ /src/api/resource/definitions/
WORKDIR /src/api
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/bufbuild/buf/cmd/buf build
RUN --mount=type=cache,target=/.cache,id=talos/.cache,sharing=locked go tool github.com/bufbuild/buf/cmd/buf generate
# Goimports and gofumpt generated files to adjust import order
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool golang.org/x/tools/cmd/goimports -w -local github.com/siderolabs/talos /src/api/machinery/
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool mvdan.cc/gofumpt -w /src/api/machinery/

FROM scratch AS generate-build-clean
COPY --link --from=generate-build /src/api /api/

FROM tools AS selinux
RUN --mount=type=bind,source=internal/pkg/selinux/policy/selinux,target=/selinux \
    mkdir /policy; secilc -o /policy/policy.33 -f /policy/file_contexts -c 33 /selinux/**/*.cil -vvvvv -O

FROM scratch AS selinux-generate
COPY --link --from=selinux /policy /policy

FROM scratch AS ipxe-generate
COPY --link --from=pkg-ipxe-amd64 /usr/libexec/snp.efi /amd64/snp.efi
COPY --link --from=pkg-ipxe-arm64 /usr/libexec/snp.efi /arm64/snp.efi

FROM scratch AS microsoft-secureboot-database
ARG MICROSOFT_SECUREBOOT_RELEASE
ADD https://github.com/microsoft/secureboot_objects.git#${MICROSOFT_SECUREBOOT_RELEASE}:PreSignedObjects /

FROM scratch AS microsoft-key-keys
COPY --link --from=microsoft-secureboot-database /KEK/Certificates/*.der /kek/

FROM scratch AS microsoft-db-keys
COPY --link --from=microsoft-secureboot-database /DB/Certificates/MicCor*.der /db/
COPY --link --from=microsoft-secureboot-database /DB/Certificates/microsoft*.der /db/

FROM --platform=${BUILDPLATFORM} scratch AS generate
COPY --link --from=proto-format-build /src/api /api/
COPY --link --from=generate-build-clean /api/resource/definitions/ /api/resource/definitions/
COPY --link --from=generate-build-clean /api/machinery /pkg/machinery/
COPY --link --from=generate-build-clean /api/docs/api.md /website/content/v1.14/reference/api.md
COPY --link --from=go-generate /src/pkg/imager/profile/ /pkg/imager/profile/
COPY --link --from=go-generate /src/pkg/machinery/resources/ /pkg/machinery/resources/
COPY --link --from=go-generate /src/pkg/machinery/config/schemas/ /pkg/machinery/config/schemas/
COPY --link --from=go-generate /src/pkg/machinery/config/types/ /pkg/machinery/config/types/
COPY --link --from=go-generate /src/pkg/machinery/imager/imageropts/ /pkg/machinery/imager/imageropts/
COPY --link --from=go-generate /src/pkg/machinery/nethelpers/ /pkg/machinery/nethelpers/
COPY --link --from=go-generate /src/pkg/machinery/extensions/ /pkg/machinery/extensions/
COPY --link --from=go-generate /src/pkg/machinery/version/os-release /pkg/machinery/version/os-release
COPY --link --from=ipxe-generate / /pkg/provision/providers/vm/internal/ipxe/data/ipxe/
COPY --link --from=selinux-generate / /internal/pkg/selinux/
COPY --link --from=embed-abbrev / /
COPY --link --from=pkg-ca-certificates /etc/ssl/certs/ca-certificates /internal/app/machined/pkg/controllers/secrets/data/
COPY --link --from=microsoft-key-keys / /internal/pkg/secureboot/database/certs/
COPY --link --from=microsoft-db-keys / /internal/pkg/secureboot/database/certs/

# The base target provides a container that can be used to build all Talos
# assets.

FROM build-go AS base
COPY ./cmd ./cmd
COPY ./pkg ./pkg
COPY ./internal ./internal
COPY --link --from=embed / ./
RUN --mount=type=cache,target=/.cache,id=talos/.cache go list all >/dev/null
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache,id=talos/.cache go list all >/dev/null
RUN --mount=type=cache,target=/.cache,id=talos/.cache go generate -v ./version
WORKDIR /src

# The vulncheck target runs the vulnerability check tool.

FROM base AS lint-vulncheck
RUN --mount=type=bind,source=.disvulncheck.yaml,target=/src/.disvulncheck.yaml --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/shanduur/dis-vulncheck ./...

# The lint-deadcode target runs the deadcode elimination check.
FROM base AS lint-deadcode
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GO_MACHINED_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} ${GO_MACHINED_LDFLAGS} -dumpdep" ./internal/app/machined \
    |& go tool github.com/aarzilli/whydeadcode > deadcode.txt
RUN if [[ -s deadcode.txt ]]; then \
    echo "Dead code elimination problem found:"; \
    cat deadcode.txt; \
    exit 1; \
    else \
    echo "No dead code elimination issues found"; \
    fi

# The init target builds the init binary.

FROM base AS init-build-amd64
WORKDIR /src/internal/app/init
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=amd64 GOAMD64=v1 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /init
RUN chmod +x /init

FROM base AS init-build-arm64
WORKDIR /src/internal/app/init
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=arm64 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /init
RUN chmod +x /init

FROM init-build-${TARGETARCH} AS init-build

FROM scratch AS init
COPY --link --from=init-build /init /init

# The machined target builds the machined binary.

FROM base AS machined-build-amd64
WORKDIR /src/internal/app/machined
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GO_MACHINED_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} ${GO_MACHINED_LDFLAGS}" -o /machined
RUN chmod +x /machined

FROM base AS machined-build-arm64
WORKDIR /src/internal/app/machined
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GO_MACHINED_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=arm64 go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS} ${GO_MACHINED_LDFLAGS}" -o /machined
RUN chmod +x /machined

FROM machined-build-${TARGETARCH} AS machined-build

FROM scratch AS machined
COPY --link --from=machined-build /machined /machined

# The labeled-squashfs target builds a build-time helper that walks the rootfs,
# resolves each path's SELinux context against file_contexts, and invokes
# mksquashfs with the labels supplied as pseudo-file definitions. This avoids
# needing fakeroot to fake security.selinux xattrs on the source tree.

FROM base AS labeled-squashfs-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /labeled-squashfs github.com/siderolabs/talos/tools/labeled-squashfs
RUN chmod +x /labeled-squashfs

# The talosctl targets build the talosctl binaries.

FROM base AS talosctl-linux-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-amd64
RUN chmod +x /talosctl-linux-amd64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-linux-amd64

FROM base AS talosctl-linux-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-arm64
RUN chmod +x /talosctl-linux-arm64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-linux-arm64

FROM base AS talosctl-linux-armv7-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=arm GOARM=7 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-armv7
RUN chmod +x /talosctl-linux-armv7
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-linux-armv7

FROM base AS talosctl-linux-riscv64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=riscv64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-linux-riscv64
RUN chmod +x /talosctl-linux-riscv64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-linux-riscv64

FROM base AS talosctl-darwin-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=darwin GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-darwin-amd64
RUN chmod +x /talosctl-darwin-amd64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-darwin-amd64

FROM base AS talosctl-darwin-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=darwin GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-darwin-arm64
RUN chmod +x /talosctl-darwin-arm64
RUN touch --date="@${SOURCE_DATE_EPOCH}" talosctl-darwin-arm64

FROM base AS talosctl-windows-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=windows GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-windows-amd64.exe
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-windows-amd64.exe

FROM base AS talosctl-windows-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=windows GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-windows-arm64.exe
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-windows-arm64.exe

FROM base AS talosctl-freebsd-amd64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=freebsd GOARCH=amd64 GOAMD64=${GOAMD64} go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-freebsd-amd64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-freebsd-amd64

FROM base AS talosctl-freebsd-arm64-build
WORKDIR /src/cmd/talosctl
ARG GO_BUILDFLAGS_TALOSCTL
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=freebsd GOARCH=arm64 go build ${GO_BUILDFLAGS_TALOSCTL} -ldflags "${GO_LDFLAGS}" -o /talosctl-freebsd-arm64
RUN touch --date="@${SOURCE_DATE_EPOCH}" /talosctl-freebsd-arm64

FROM scratch AS talosctl-linux-amd64
COPY --link --from=talosctl-linux-amd64-build /talosctl-linux-amd64 /talosctl-linux-amd64

FROM scratch AS talosctl-linux-arm64
COPY --link --from=talosctl-linux-arm64-build /talosctl-linux-arm64 /talosctl-linux-arm64

FROM scratch AS talosctl-linux-armv7
COPY --link --from=talosctl-linux-armv7-build /talosctl-linux-armv7 /talosctl-linux-armv7

FROM scratch AS talosctl-linux-riscv64
COPY --link --from=talosctl-linux-riscv64-build /talosctl-linux-riscv64 /talosctl-linux-riscv64

FROM scratch AS talosctl-darwin-amd64
COPY --link --from=talosctl-darwin-amd64-build /talosctl-darwin-amd64 /talosctl-darwin-amd64

FROM scratch AS talosctl-darwin-arm64
COPY --link --from=talosctl-darwin-arm64-build /talosctl-darwin-arm64 /talosctl-darwin-arm64

FROM scratch AS talosctl-freebsd-amd64
COPY --link --from=talosctl-freebsd-amd64-build /talosctl-freebsd-amd64 /talosctl-freebsd-amd64

FROM scratch AS talosctl-freebsd-arm64
COPY --link --from=talosctl-freebsd-arm64-build /talosctl-freebsd-arm64 /talosctl-freebsd-arm64

FROM scratch AS talosctl-windows-amd64
COPY --link --from=talosctl-windows-amd64-build /talosctl-windows-amd64.exe /talosctl-windows-amd64.exe

FROM scratch AS talosctl-windows-arm64
COPY --link --from=talosctl-windows-arm64-build /talosctl-windows-arm64.exe /talosctl-windows-arm64.exe

FROM --platform=${BUILDPLATFORM} talosctl-${TARGETOS}-${TARGETARCH} AS talosctl-targetarch

FROM scratch AS talosctl-all
COPY --link --from=talosctl-linux-amd64 / /
COPY --link --from=talosctl-linux-arm64 / /
COPY --link --from=talosctl-linux-armv7 / /
COPY --link --from=talosctl-linux-riscv64 / /
COPY --link --from=talosctl-darwin-amd64 / /
COPY --link --from=talosctl-darwin-arm64 / /
COPY --link --from=talosctl-freebsd-amd64 / /
COPY --link --from=talosctl-freebsd-arm64 / /
COPY --link --from=talosctl-windows-amd64 / /
COPY --link --from=talosctl-windows-arm64 / /

FROM scratch AS talosctl
ARG TARGETARCH
COPY --link --from=talosctl-all /talosctl-linux-${TARGETARCH} /talosctl
ARG TAG
ENV VERSION=${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
LABEL org.opencontainers.image.source=https://github.com/siderolabs/talos
ENTRYPOINT ["/talosctl"]

# The kernel target is the linux kernel.
FROM scratch AS kernel
ARG TARGETARCH
COPY --link --from=pkg-kernel /boot/vmlinuz /vmlinuz-${TARGETARCH}

# The sd-boot target is the systemd-boot asset.
FROM scratch AS sd-boot
ARG TARGETARCH
COPY --link --from=pkg-sd-boot /*.efi /sd-boot-${TARGETARCH}.efi

# The sd-stub target is the systemd-stub asset.
FROM scratch AS sd-stub
ARG TARGETARCH
COPY --link --from=pkg-sd-boot /*.efi.stub /sd-stub-${TARGETARCH}.efi

FROM tools AS depmod-amd64
WORKDIR /staging
COPY --link --from=pkg-kernel-amd64 /usr/lib/modules usr/lib/modules
COPY --link --from=pkg-kernel-amd64 /boot/System.map /staging/
RUN --mount=type=bind,source=hack/modules-amd64.txt,target=/staging/modules-amd64.txt <<EOF
set -euo pipefail

KERNEL_VERSION=$(ls usr/lib/modules)

xargs -a modules-amd64.txt -I {} install -D usr/lib/modules/${KERNEL_VERSION}/{} /build/usr/lib/modules/${KERNEL_VERSION}/{}

# check if the output of the command is empty, as depmod doesn't fail and just prints a warning
DEPMOD_OUTPUT=$(depmod -b /build/usr -F /staging/System.map --errsyms -w ${KERNEL_VERSION} 2>&1)

if [ -n "${DEPMOD_OUTPUT}" ]; then
    echo "depmod output is not empty, indicating a potential issue:"
    echo "${DEPMOD_OUTPUT}"
    exit 1
else
    echo "depmod completed successfully with no warnings."
fi

EOF

FROM scratch AS modules-amd64
COPY --link --from=depmod-amd64 /build/usr/lib/modules /usr/lib/modules

FROM tools AS depmod-arm64
WORKDIR /staging
COPY --link --from=pkg-kernel-arm64 /usr/lib/modules usr/lib/modules
COPY --link --from=pkg-kernel-arm64 /boot/System.map /staging/
RUN --mount=type=bind,source=hack/modules-arm64.txt,target=/staging/modules-arm64.txt <<EOF
set -euo pipefail

KERNEL_VERSION=$(ls usr/lib/modules)

xargs -a modules-arm64.txt -I {} install -D usr/lib/modules/${KERNEL_VERSION}/{} /build/usr/lib/modules/${KERNEL_VERSION}/{}

# check if the output of the command is empty, as depmod doesn't fail and just prints a warning
DEPMOD_OUTPUT=$(depmod -b /build/usr -F /staging/System.map --errsyms -w ${KERNEL_VERSION} 2>&1)

if [ -n "${DEPMOD_OUTPUT}" ]; then
    echo "depmod output is not empty, indicating a potential issue:"
    echo "${DEPMOD_OUTPUT}"
    exit 1
else
    echo "depmod completed successfully with no warnings."
fi

EOF

FROM scratch AS modules-arm64
COPY --link --from=depmod-arm64 /build/usr/lib/modules /usr/lib/modules

# The rootfs target provides the Talos rootfs.
FROM tools AS rootfs-base-amd64
SHELL ["/bin/bash", "-c"]
COPY --link --from=pkg-fhs / /rootfs
COPY --link --from=pkg-apparmor-amd64 / /rootfs
COPY --link --from=pkg-cni-stripped-amd64 / /rootfs
COPY --link --from=pkg-flannel-cni-amd64 / /rootfs
COPY --link --from=pkg-cryptsetup-amd64 / /rootfs
COPY --link --exclude=usr/bin/ctr --from=pkg-containerd-amd64 / /rootfs
COPY --link --from=pkg-dosfstools-amd64 / /rootfs
COPY --link --from=pkg-e2fsprogs-amd64 / /rootfs
COPY --link --exclude=usr/share --from=pkg-systemd-udevd-amd64 / /rootfs
COPY --link --from=pkg-systemd-udevd-amd64 /usr/share/spdx/systemd.spdx.json /rootfs/usr/share/spdx/systemd.spdx.json
COPY --link --from=pkg-libcap-amd64 / /rootfs
COPY --link --exclude=usr/share --from=pkg-iptables-amd64 / /rootfs
COPY --link --from=pkg-iptables-amd64 /usr/share/spdx/iptables.spdx.json /rootfs/usr/share/spdx/iptables.spdx.json
COPY --link --from=pkg-libarchive-amd64 / /rootfs
COPY --link --from=pkg-libattr-amd64 / /rootfs
COPY --link --from=pkg-libinih-amd64 / /rootfs
COPY --link --exclude=usr/include --from=pkg-libjansson-amd64 / /rootfs
COPY --link --from=pkg-libjson-c-amd64 / /rootfs
COPY --link --from=pkg-libmnl-amd64 / /rootfs
COPY --link --from=pkg-libnftnl-amd64 / /rootfs
COPY --link --from=pkg-libpopt-amd64 / /rootfs
COPY --link --from=pkg-liburcu-amd64 / /rootfs
COPY --link --from=pkg-libsepol-amd64 / /rootfs
COPY --link --from=pkg-libselinux-amd64 / /rootfs
COPY --link --from=pkg-zstd-amd64 /usr/share/spdx /rootfs/usr/share/spdx
COPY --link --from=pkg-zstd-amd64 /usr/lib /rootfs/usr/lib
COPY --link --from=pkg-zlib-amd64 /usr/share/spdx /rootfs/usr/share/spdx
COPY --link --from=pkg-zlib-amd64 /usr/lib /rootfs/usr/lib
# NOTE: amd64 ships igzip, but arm64 ships pigz (see https://github.com/siderolabs/extensions/discussions/931)
COPY --link --exclude=usr/lib/pkgconfig --exclude=usr/include --from=pkg-igzip-amd64 / /rootfs
COPY --link --from=pkg-pcre2-amd64 / /rootfs
COPY --link --from=pkg-openssl-amd64 / /rootfs
COPY --link --from=pkg-lvm2-amd64 / /rootfs
COPY --link --from=pkg-libaio-amd64 / /rootfs
COPY --link --from=pkg-musl-amd64 / /rootfs
COPY --link --from=pkg-nftables-amd64 / /rootfs
COPY --link --from=pkg-runc-amd64 / /rootfs
COPY --link --from=pkg-xfsprogs-amd64 / /rootfs
COPY --link --from=pkg-util-linux-amd64 /usr/lib/libblkid.* /rootfs/usr/lib/
COPY --link --from=pkg-util-linux-amd64 /usr/lib/libuuid.* /rootfs/usr/lib/
COPY --link --from=pkg-util-linux-amd64 /usr/lib/libmount.* /rootfs/usr/lib/
COPY --link --from=pkg-util-linux-amd64 /usr/share/spdx/util-linux.spdx.json /rootfs/usr/share/spdx/util-linux.spdx.json
COPY --link --from=pkg-kmod-amd64 /usr/lib/libkmod.* /rootfs/usr/lib/
COPY --link --from=pkg-kmod-amd64 /usr/bin/kmod /rootfs/usr/bin/modprobe
COPY --link --from=pkg-kmod-amd64 usr/share/spdx/kmod.spdx.json /rootfs/usr/share/spdx/kmod.spdx.json
COPY --link --from=modules-amd64 /usr/lib/modules /rootfs/usr/lib/modules
COPY --link --from=machined-build-amd64 /machined /rootfs/usr/bin/init

RUN <<END
    # the orderly_poweroff call by the kernel will call '/sbin/poweroff'
    ln /rootfs/usr/bin/init /rootfs/usr/bin/poweroff
    chmod +x /rootfs/usr/bin/poweroff
    # some extensions like qemu-guest agent will call '/sbin/shutdown'
    ln /rootfs/usr/bin/init /rootfs/usr/bin/shutdown
    chmod +x /rootfs/usr/bin/shutdown
    ln /rootfs/usr/bin/init /rootfs/usr/bin/dashboard
    chmod +x /rootfs/usr/bin/dashboard
END
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
RUN --mount=type=bind,source=hack/cleanup.sh,target=/usr/bin/cleanup.sh <<END
    cleanup.sh /rootfs
    mkdir -pv /rootfs/{boot/EFI,etc/{iscsi,nvme,cri/conf.d/hosts},usr/lib/firmware,usr/etc,usr/local/share,usr/share/zoneinfo/Etc,mnt,system,opt,.extra}
    mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni/net.d,etc/ssl/certs,usr/libexec/kubernetes,/usr/local/lib/kubelet/credentialproviders,etc/selinux/targeted/contexts/files}
    mkdir -pv /rootfs/opt/{containerd/bin,containerd/lib}
    # Go standard library is shipped with Talos, thus it must be tracked in SBOM
    install -D /usr/share/spdx/golang.spdx.json /rootfs/usr/share/spdx/golang.spdx.json
END
COPY --chmod=0644 hack/zoneinfo/Etc/UTC /rootfs/usr/share/zoneinfo/Etc/UTC
COPY --chmod=0644 hack/nfsmount.conf /rootfs/etc/nfsmount.conf
COPY --chmod=0644 hack/containerd.toml /rootfs/etc/containerd/config.toml
COPY --chmod=0644 hack/cri-containerd.toml /rootfs/etc/cri/containerd.toml
COPY --chmod=0644 hack/cri-plugin.part /rootfs/etc/cri/conf.d/00-base.part
COPY --chmod=0644 hack/udevd/99-default.link /rootfs/usr/lib/systemd/network/
COPY --chmod=0644 hack/udevd/40-vm-hotadd.rules hack/udevd/90-selinux.rules /rootfs/usr/lib/udev/rules.d/
COPY --chmod=0644 hack/lvm.conf /rootfs/etc/lvm/lvm.conf
COPY --link --chmod=0644 --from=base /src/pkg/machinery/version/os-release /rootfs/etc/os-release
RUN <<END
    ln -s /usr/share/zoneinfo/Etc/UTC /rootfs/etc/localtime
    touch /rootfs/etc/{extensions.yaml,resolv.conf,hosts,machine-id,cri/conf.d/cri.toml,cri/conf.d/01-registries.part,cri/conf.d/20-customization.part,cri/conf.d/base-spec.json,ssl/certs/ca-certificates.crt,selinux/targeted/contexts/files/file_contexts,iscsi/initiatorname.iscsi,nvme/{hostid,hostnqn}}
    ln -s ca-certificates.crt /rootfs/etc/ssl/certs/ca-certificates
    ln -s /etc/ssl /rootfs/etc/pki
    ln -s /etc/ssl /rootfs/usr/share/ca-certificates
    ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
    ln -s /etc/ssl /rootfs/etc/ca-certificates
END

FROM tools AS rootfs-base-arm64
SHELL ["/bin/bash", "-c"]
COPY --link --from=pkg-fhs / /rootfs
COPY --link --from=pkg-apparmor-arm64 / /rootfs
COPY --link --from=pkg-cni-stripped-arm64 / /rootfs
COPY --link --from=pkg-flannel-cni-arm64 / /rootfs
COPY --link --from=pkg-cryptsetup-arm64 / /rootfs
COPY --link --exclude=usr/bin/ctr --from=pkg-containerd-arm64 / /rootfs
COPY --link --from=pkg-dosfstools-arm64 / /rootfs
COPY --link --from=pkg-e2fsprogs-arm64 / /rootfs
COPY --link --exclude=usr/share --from=pkg-systemd-udevd-arm64 / /rootfs
COPY --link --from=pkg-systemd-udevd-arm64 /usr/share/spdx/systemd.spdx.json /rootfs/usr/share/spdx/systemd.spdx.json
COPY --link --from=pkg-libcap-arm64 / /rootfs
COPY --link --exclude=usr/share --from=pkg-iptables-arm64 / /rootfs
COPY --link --from=pkg-iptables-arm64 /usr/share/spdx/iptables.spdx.json /rootfs/usr/share/spdx/iptables.spdx.json
COPY --link --from=pkg-libarchive-arm64 / /rootfs
COPY --link --from=pkg-libattr-arm64 / /rootfs
COPY --link --from=pkg-libinih-arm64 / /rootfs
COPY --link --exclude=usr/include --from=pkg-libjansson-arm64 / /rootfs
COPY --link --from=pkg-libjson-c-arm64 / /rootfs
COPY --link --from=pkg-libmnl-arm64 / /rootfs
COPY --link --from=pkg-libnftnl-arm64 / /rootfs
COPY --link --from=pkg-libpopt-arm64 / /rootfs
COPY --link --from=pkg-liburcu-arm64 / /rootfs
COPY --link --from=pkg-libsepol-arm64 / /rootfs
COPY --link --from=pkg-libselinux-arm64 / /rootfs
COPY --link --from=pkg-pcre2-arm64 / /rootfs
COPY --link --from=pkg-openssl-arm64 / /rootfs
COPY --link --from=pkg-lvm2-arm64 / /rootfs
COPY --link --from=pkg-libaio-arm64 / /rootfs
COPY --link --from=pkg-musl-arm64 / /rootfs
COPY --link --from=pkg-nftables-arm64 / /rootfs
COPY --link --from=pkg-runc-arm64 / /rootfs
COPY --link --from=pkg-xfsprogs-arm64 / /rootfs
COPY --link --from=pkg-zstd-arm64 /usr/share/spdx /rootfs/usr/share/spdx
COPY --link --from=pkg-zstd-arm64 /usr/lib /rootfs/usr/lib
COPY --link --from=pkg-zlib-arm64 /usr/share/spdx /rootfs/usr/share/spdx
COPY --link --from=pkg-zlib-arm64 /usr/lib /rootfs/usr/lib
# NOTE: amd64 ships igzip, but arm64 ships pigz (see https://github.com/siderolabs/extensions/discussions/931)
COPY --link --from=pkg-pigz-arm64 / /rootfs
COPY --link --from=pkg-util-linux-arm64 /usr/lib/libblkid.* /rootfs/usr/lib/
COPY --link --from=pkg-util-linux-arm64 /usr/lib/libuuid.* /rootfs/usr/lib/
COPY --link --from=pkg-util-linux-arm64 /usr/lib/libmount.* /rootfs/usr/lib/
COPY --link --from=pkg-util-linux-arm64 /usr/share/spdx/util-linux.spdx.json /rootfs/usr/share/spdx/util-linux.spdx.json
COPY --link --from=pkg-kmod-arm64 /usr/lib/libkmod.* /rootfs/usr/lib/
COPY --link --from=pkg-kmod-arm64 /usr/bin/kmod /rootfs/usr/bin/modprobe
COPY --link --from=pkg-kmod-arm64 /usr/share/spdx/kmod.spdx.json /rootfs/usr/share/spdx/kmod.spdx.json
COPY --link --from=modules-arm64 /usr/lib/modules /rootfs/usr/lib/modules
COPY --link --from=machined-build-arm64 /machined /rootfs/usr/bin/init

RUN <<END
    # the orderly_poweroff call by the kernel will call '/sbin/poweroff'
    ln /rootfs/usr/bin/init /rootfs/usr/bin/poweroff
    chmod +x /rootfs/usr/bin/poweroff
    # some extensions like qemu-guest agent will call '/sbin/shutdown'
    ln /rootfs/usr/bin/init /rootfs/usr/bin/shutdown
    chmod +x /rootfs/usr/bin/shutdown
    ln /rootfs/usr/bin/init /rootfs/usr/bin/dashboard
    chmod +x /rootfs/usr/bin/dashboard
END
# NB: We run the cleanup step before creating extra directories, files, and
# symlinks to avoid accidentally cleaning them up.
RUN --mount=type=bind,source=hack/cleanup.sh,target=/usr/bin/cleanup.sh <<END
    cleanup.sh /rootfs
    mkdir -pv /rootfs/{boot/EFI,etc/{iscsi,nvme,cri/conf.d/hosts},usr/lib/firmware,usr/etc,usr/local/share,usr/share/zoneinfo/Etc,mnt,system,opt,.extra}
    mkdir -pv /rootfs/{etc/kubernetes/manifests,etc/cni/net.d,etc/ssl/certs,usr/libexec/kubernetes,/usr/local/lib/kubelet/credentialproviders,etc/selinux/targeted/contexts/files}
    mkdir -pv /rootfs/opt/{containerd/bin,containerd/lib}
    # Go standard library is shipped with Talos, thus it must be tracked in SBOM
    install -D /usr/share/spdx/golang.spdx.json /rootfs/usr/share/spdx/golang.spdx.json
END
COPY --chmod=0644 hack/zoneinfo/Etc/UTC /rootfs/usr/share/zoneinfo/Etc/UTC
COPY --chmod=0644 hack/nfsmount.conf /rootfs/etc/nfsmount.conf
COPY --chmod=0644 hack/containerd.toml /rootfs/etc/containerd/config.toml
COPY --chmod=0644 hack/cri-containerd.toml /rootfs/etc/cri/containerd.toml
COPY --chmod=0644 hack/cri-plugin.part /rootfs/etc/cri/conf.d/00-base.part
COPY --chmod=0644 hack/udevd/99-default.link /rootfs/usr/lib/systemd/network/
COPY --chmod=0644 hack/udevd/40-vm-hotadd.rules hack/udevd/90-selinux.rules /rootfs/usr/lib/udev/rules.d/
COPY --chmod=0644 hack/lvm.conf /rootfs/etc/lvm/lvm.conf
COPY --link --chmod=0644 --from=base /src/pkg/machinery/version/os-release /rootfs/etc/os-release
RUN <<END
    ln -s /usr/share/zoneinfo/Etc/UTC /rootfs/etc/localtime
    touch /rootfs/etc/{extensions.yaml,resolv.conf,hosts,machine-id,cri/conf.d/cri.toml,cri/conf.d/01-registries.part,cri/conf.d/20-customization.part,cri/conf.d/base-spec.json,ssl/certs/ca-certificates.crt,selinux/targeted/contexts/files/file_contexts,iscsi/initiatorname.iscsi,nvme/{hostid,hostnqn}}
    ln -s ca-certificates.crt /rootfs/etc/ssl/certs/ca-certificates
    ln -s /etc/ssl /rootfs/etc/pki
    ln -s /etc/ssl /rootfs/usr/share/ca-certificates
    ln -s /etc/ssl /rootfs/usr/local/share/ca-certificates
    ln -s /etc/ssl /rootfs/etc/ca-certificates
END

FROM build-go AS build-sbom
ARG SOURCE_DATE_EPOCH
ARG NAME
ARG TAG

FROM build-sbom AS sbom-container-arm64-generate
RUN --mount=type=tmpfs,target=/tmp/sbom-src \
    --mount=type=bind,from=rootfs-base-arm64,source=/rootfs/usr/share/spdx,target=/mnt/spdx \
    --mount=type=cache,target=/.cache,id=talos/.cache <<EOF
set -euo pipefail
mkdir -p /rootfs/usr/share/spdx
cp -r /mnt/spdx/. /tmp/sbom-src/
cp go.mod go.sum /tmp/sbom-src/
go tool github.com/siderolabs/talos/tools/sbom-builder \
    --source-dir /tmp/sbom-src/ \
    --source-name "$NAME" \
    --source-version "$TAG" \
    --source-date-epoch "${SOURCE_DATE_EPOCH:-0}" \
    --output /rootfs/usr/share/spdx/talos-container-arm64.spdx.json
EOF

FROM scratch AS sbom-container-arm64
COPY --link --from=sbom-container-arm64-generate /rootfs/usr/share/spdx/talos-container-arm64.spdx.json /

FROM build-sbom AS sbom-container-amd64-generate
RUN --mount=type=tmpfs,target=/tmp/sbom-src \
    --mount=type=bind,from=rootfs-base-amd64,source=/rootfs/usr/share/spdx,target=/mnt/spdx \
    --mount=type=cache,target=/.cache,id=talos/.cache <<EOF
set -euo pipefail
mkdir -p /rootfs/usr/share/spdx
cp -r /mnt/spdx/. /tmp/sbom-src/
cp go.mod go.sum /tmp/sbom-src/
go tool github.com/siderolabs/talos/tools/sbom-builder \
    --source-dir /tmp/sbom-src/ \
    --source-name "$NAME" \
    --source-version "$TAG" \
    --source-date-epoch "${SOURCE_DATE_EPOCH:-0}" \
    --output /rootfs/usr/share/spdx/talos-container-amd64.spdx.json
EOF

FROM scratch AS sbom-container-amd64
COPY --link --from=sbom-container-amd64-generate /rootfs/usr/share/spdx/talos-container-amd64.spdx.json /

FROM build-sbom AS sbom-arm64-generate
RUN --mount=type=tmpfs,target=/tmp/sbom-src \
    --mount=type=bind,from=rootfs-base-arm64,source=/rootfs/usr/share/spdx,target=/mnt/spdx \
    --mount=type=bind,from=pkg-kernel-arm64,source=/usr/share/spdx/kernel.spdx.json,target=/mnt/kernel.spdx.json \
    --mount=type=cache,target=/.cache,id=talos/.cache <<EOF
set -euo pipefail
mkdir -p /rootfs/usr/share/spdx
cp -r /mnt/spdx/. /tmp/sbom-src/
cp /mnt/kernel.spdx.json /tmp/sbom-src/
cp go.mod go.sum /tmp/sbom-src/
go tool github.com/siderolabs/talos/tools/sbom-builder \
    --source-dir /tmp/sbom-src/ \
    --source-name "$NAME" \
    --source-version "$TAG" \
    --source-date-epoch "${SOURCE_DATE_EPOCH:-0}" \
    --output /rootfs/usr/share/spdx/talos-arm64.spdx.json
EOF

FROM scratch AS sbom-arm64
COPY --link --from=sbom-arm64-generate /rootfs/usr/share/spdx/talos-arm64.spdx.json /

FROM build-sbom AS sbom-amd64-generate
RUN --mount=type=tmpfs,target=/tmp/sbom-src \
    --mount=type=bind,from=rootfs-base-amd64,source=/rootfs/usr/share/spdx,target=/mnt/spdx \
    --mount=type=bind,from=pkg-kernel-amd64,source=/usr/share/spdx/kernel.spdx.json,target=/mnt/kernel.spdx.json \
    --mount=type=cache,target=/.cache,id=talos/.cache <<EOF
set -euo pipefail
mkdir -p /rootfs/usr/share/spdx
cp -r /mnt/spdx/. /tmp/sbom-src/
cp /mnt/kernel.spdx.json /tmp/sbom-src/
cp go.mod go.sum /tmp/sbom-src/
go tool github.com/siderolabs/talos/tools/sbom-builder \
    --source-dir /tmp/sbom-src/ \
    --source-name "$NAME" \
    --source-version "$TAG" \
    --source-date-epoch "${SOURCE_DATE_EPOCH:-0}" \
    --output /rootfs/usr/share/spdx/talos-amd64.spdx.json
EOF

FROM scratch AS sbom-amd64
COPY --link --from=sbom-amd64-generate /rootfs/usr/share/spdx/talos-amd64.spdx.json /

FROM scratch AS sbom
COPY --link --from=sbom-container-arm64 / /
COPY --link --from=sbom-container-amd64 / /
COPY --link --from=sbom-arm64 / /
COPY --link --from=sbom-amd64 / /

FROM sbom-container-${TARGETARCH} AS sbom-container-target

# Use an unpinned latest version, because we want to use the latest advisories
FROM ${GENERATE_VEX_PREFIX}:${GENERATE_VEX} AS talos-vex

FROM build-go AS vex-generate
ARG TAG
RUN --mount=type=bind,from=talos-vex,source=/generate-vex,target=/generate-vex /generate-vex gen --target-version $TAG > /talos.vex.json
# This config contains IDs of the tracked, but affected vulnerabilities.
# Once an advisory is made, the CI should go back to passing status.
RUN --mount=type=bind,from=talos-vex,source=/generate-vex,target=/generate-vex /generate-vex grype-config --target-version $TAG > /talos.grype.yaml

FROM scratch AS vex
COPY --link --from=vex-generate /talos.vex.json /talos.vex.json
COPY --link --from=vex-generate /talos.grype.yaml /talos.grype.yaml

FROM build-go AS grype-scan
COPY --link --from=sbom-arm64 /talos-arm64.spdx.json /talos-arm64.spdx.json
COPY --link --from=vex /talos.vex.json /talos.vex.json
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool \
    github.com/anchore/grype/cmd/grype sbom:/talos-arm64.spdx.json \
    --vex /talos.vex.json -vv 2>&1 | tee /grype-scan.log

FROM scratch AS grype-scan-result
COPY --link --from=grype-scan /grype-scan.log /grype-scan.log

FROM build-go AS grype-validate
COPY --link --from=sbom-arm64 /talos-arm64.spdx.json /talos-arm64.spdx.json
COPY --link --from=vex /talos.vex.json /talos.vex.json
COPY --link --from=vex /talos.grype.yaml /talos.grype.yaml
RUN --mount=type=cache,target=/.cache,id=talos/.cache go tool \
    github.com/anchore/grype/cmd/grype sbom:/talos-arm64.spdx.json \
    --vex /talos.vex.json -vv --fail-on negligible --config /talos.grype.yaml

FROM rootfs-base-${TARGETARCH} AS rootfs-base
RUN rm -rf /rootfs/usr/share/spdx/*
COPY --link --from=sbom-container-target / /rootfs/usr/share/spdx/
RUN echo "true" > /rootfs/usr/etc/in-container
RUN rm -rf /rootfs/usr/lib/modules/*
ARG SOURCE_DATE_EPOCH
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"

FROM rootfs-base-arm64 AS rootfs-squashfs-arm64
RUN rm -rf /rootfs/usr/share/spdx/*
COPY --link --from=sbom-arm64 / /rootfs/usr/share/spdx/
ARG SOURCE_DATE_EPOCH
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
ARG ZSTD_COMPRESSION_LEVEL
COPY --link --from=selinux-generate /policy/file_contexts /file_contexts
RUN --mount=from=labeled-squashfs-build,source=/labeled-squashfs,target=/usr/local/bin/labeled-squashfs \
    labeled-squashfs /rootfs /rootfs.sqsh /file_contexts ${ZSTD_COMPRESSION_LEVEL}

FROM rootfs-base-amd64 AS rootfs-squashfs-amd64
RUN rm -rf /rootfs/usr/share/spdx/*
COPY --link --from=sbom-amd64 / /rootfs/usr/share/spdx/
ARG SOURCE_DATE_EPOCH
RUN find /rootfs -print0 \
    | xargs -0r touch --no-dereference --date="@${SOURCE_DATE_EPOCH}"
ARG ZSTD_COMPRESSION_LEVEL
COPY --link --from=selinux-generate /policy/file_contexts /file_contexts
RUN --mount=from=labeled-squashfs-build,source=/labeled-squashfs,target=/usr/local/bin/labeled-squashfs \
    labeled-squashfs /rootfs /rootfs.sqsh /file_contexts ${ZSTD_COMPRESSION_LEVEL}

FROM scratch AS squashfs-arm64
COPY --link --from=rootfs-squashfs-arm64 /rootfs.sqsh /

FROM scratch AS squashfs-amd64
COPY --link --from=rootfs-squashfs-amd64 /rootfs.sqsh /

FROM scratch AS rootfs
COPY --link --from=rootfs-base /rootfs /

# The initramfs target provides the Talos initramfs image.

FROM build AS initramfs-archive-arm64
WORKDIR /initramfs
ARG ZSTD_COMPRESSION_LEVEL
COPY --link --from=squashfs-arm64 /rootfs.sqsh .
COPY --link --from=init-build-arm64 /init .
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
COPY --link --from=squashfs-amd64 /rootfs.sqsh .
COPY --link --from=init-build-amd64 /init .
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
COPY --link --from=initramfs-archive /initramfs.xz /initramfs-${TARGETARCH}.xz

# The talos target generates a docker image that can be used to run Talos
# in containers.

FROM scratch AS talos
COPY --link --from=rootfs / /
LABEL org.opencontainers.image.source=https://github.com/siderolabs/talos
ENTRYPOINT ["/sbin/init"]

# The installer target generates an image that can be used to install Talos to
# various environments.

# Make the installer binary.
FROM base AS installer-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
WORKDIR /src/cmd/installer
ARG TARGETARCH
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=${TARGETARCH} go build ${GO_BUILDFLAGS} -ldflags "${GO_LDFLAGS}" -o /installer
RUN chmod +x /installer

# Make the images containing the boot artifacts.
FROM scratch AS install-artifacts-amd64
COPY --link --from=pkg-kernel-amd64 /boot/vmlinuz /usr/install/amd64/vmlinuz
COPY --link --from=initramfs-archive-amd64 /initramfs.xz /usr/install/amd64/initramfs.xz
COPY --link --from=pkg-sd-boot-amd64 /linuxx64.efi.stub /usr/install/amd64/systemd-stub.efi
COPY --link --from=pkg-sd-boot-amd64 /systemd-bootx64.efi /usr/install/amd64/systemd-boot.efi
COPY --link --from=sbom-amd64 /talos-amd64.spdx.json /usr/install/amd64/talos.spdx.json

FROM scratch AS install-artifacts-arm64
COPY --link --from=pkg-kernel-arm64 /boot/vmlinuz /usr/install/arm64/vmlinuz
COPY --link --from=initramfs-archive-arm64 /initramfs.xz /usr/install/arm64/initramfs.xz
COPY --link --from=pkg-sd-boot-arm64 /linuxaa64.efi.stub /usr/install/arm64/systemd-stub.efi
COPY --link --from=pkg-sd-boot-arm64 /systemd-bootaa64.efi /usr/install/arm64/systemd-boot.efi
COPY --link --from=sbom-arm64 /talos-arm64.spdx.json /usr/install/arm64/talos.spdx.json

FROM scratch AS install-artifacts-all
COPY --link --from=install-artifacts-amd64 / /
COPY --link --from=install-artifacts-arm64 / /

FROM install-artifacts-${TARGETARCH} AS install-artifacts-targetarch

FROM install-artifacts-${INSTALLER_ARCH} AS install-artifacts

# Add the installer with a symlink as 'imager' and a /rootfs dir containing only the installer.
FROM tools AS installer-image-gen
COPY --link --from=installer-build /installer /rootfs/usr/bin/installer
RUN ln -s installer /rootfs/usr/bin/imager

# Add the installer binary and the tools needed to run the installer.
FROM scratch AS installer-base-image
ARG TARGETARCH
ENV TARGETARCH=${TARGETARCH}
COPY --link --from=pkg-fhs / /
COPY --link --from=pkg-ca-certificates / /
COPY --link --exclude=**/*.a --exclude=**/*.la --exclude=usr/include --from=pkg-musl / /
COPY --link --from=pkg-dosfstools / /
COPY --link --exclude=etc/bash_completion.d --from=pkg-grub / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-libattr / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-libinih / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-liblzma / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-liburcu / /
COPY --link --from=pkg-mtools / /
COPY --link --from=pkg-xfsprogs / /
# Only copy the installer binary and none of the tools used for building it.
COPY --link --from=installer-image-gen /rootfs /

# Squash the installer-base-image layers to reduce size.
FROM scratch AS installer-base-image-squashed
COPY --link --from=installer-base-image / /

# Add metadata.
# 'installer-base' only contains the installer binary and the tools it uses.
# 'installer-base' does not contain boot assets or talos itself.
FROM installer-base-image-squashed AS installer-base
ARG TAG
ENV VERSION=${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
LABEL org.opencontainers.image.source=https://github.com/siderolabs/talos
ENTRYPOINT ["/bin/installer"]

# Imager can be thought of as an extended installer.
# It has the boot artifacts and tools to build any requested talos image with desired modifications and system extensions.
# Imager is meant to be run outside of talos and the talos installation flow.
FROM installer-base-image-squashed AS imager-image
COPY --link --from=pkg-cpio / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/lib/pkgconfig --from=pkg-e2fsprogs / /
COPY --link --exclude=**/*.a --exclude=**/*.la --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-glib / /
COPY --link --from=pkg-grub-amd64 /usr/lib/grub /usr/lib/grub
COPY --link --from=pkg-grub-arm64 /usr/lib/grub /usr/lib/grub
COPY --link --exclude=usr/include --exclude=usr/lib/pkgconfig --exclude=usr/share/pkgconfig --exclude=usr/share/bash-completion --from=pkg-kmod / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-libarchive / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-libburn / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-libisoburn / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-libisofs / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --exclude=usr/lib/cmake --from=pkg-openssl / /
COPY --link --from=pkg-open-vmdk / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-pcre2 / /
COPY --link --from=pkg-pigz / /
COPY --link --from=pkg-qemu-tools / /
COPY --link --from=pkg-squashfs-tools / /
COPY --link --from=pkg-tar / /
COPY --link --exclude=**/*.a --exclude=*.a --from=pkg-util-linux /usr/lib/libblkid.* /usr/lib/
COPY --link --exclude=**/*.a --exclude=*.a --from=pkg-util-linux /usr/lib/libuuid.* /usr/lib/
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-xz / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-zlib / /
COPY --link --exclude=**/*.a --exclude=**/*.la  --exclude=usr/include --exclude=usr/lib/pkgconfig --from=pkg-zstd / /
COPY --chmod=0644 hack/extra-modules.conf /etc/modules.d/10-extra-modules.conf
COPY --link --from=install-artifacts / /

FROM scratch AS imager-image-squashed
COPY --link --from=imager-image / /

FROM imager-image-squashed AS imager
ARG TAG
ENV VERSION=${TAG}
LABEL "alpha.talos.dev/version"="${VERSION}"
LABEL org.opencontainers.image.source=https://github.com/siderolabs/talos
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
COPY --link --from=iso-amd64-build /out /

FROM scratch AS iso-arm64
COPY --link --from=iso-arm64-build /out /

FROM --platform=${BUILDPLATFORM} iso-${TARGETARCH} AS iso

# The test target performs tests on the source code.
FROM base AS unit-tests-runner
COPY --link --from=rootfs / /
COPY --link --from=pkg-ca-certificates / /
COPY --link --from=pkg-btrfsprogs / /
ARG TESTPKGS
ENV PLATFORM=container
ARG GO_LDFLAGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache,id=talos/.cache go test \
    -ldflags "${GO_LDFLAGS}" \
    -covermode=atomic -coverprofile=coverage.txt -coverpkg=${TESTPKGS} -p 4 ${TESTPKGS}
FROM scratch AS unit-tests
COPY --link --from=unit-tests-runner /src/coverage.txt /coverage.txt

# The unit-tests-race target performs tests with race detector.

FROM base AS unit-tests-race
COPY --link --from=rootfs / /
COPY --link --from=pkg-ca-certificates / /
COPY --link --from=pkg-btrfsprogs / /
ARG TESTPKGS
ENV PLATFORM=container
ENV CGO_ENABLED=1
ARG GO_LDFLAGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache,id=talos/.cache go test \
    -ldflags "${GO_LDFLAGS}" \
    -race -p 4 ${TESTPKGS}

# The unit-tests-fips target performs tests with FIPS strict mode.
FROM base AS unit-tests-fips
COPY --link --from=rootfs / /
COPY --link --from=pkg-ca-certificates / /
COPY --link --from=pkg-btrfsprogs / /
ARG TESTPKGS
ENV PLATFORM=container
ENV GOFIPS140=latest
ENV GODEBUG=fips140=only,tlsmlkem=0
ARG GO_LDFLAGS
RUN --security=insecure --mount=type=cache,id=testspace,target=/tmp --mount=type=cache,target=/.cache,id=talos/.cache go test \
    -ldflags "${GO_LDFLAGS}" \
    -p 4 ${TESTPKGS}

# The integration-test targets builds integration test binary.

FROM base AS integration-test-linux-amd64-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-linux-amd64
COPY --link --from=integration-test-linux-amd64-build /src/integration.test /integration-test-linux-amd64

FROM base AS integration-test-linux-arm64-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=arm64 go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-linux-arm64
COPY --link --from=integration-test-linux-arm64-build /src/integration.test /integration-test-linux-arm64

FROM base AS integration-test-darwin-arm64-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=darwin GOARCH=arm64 go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_api,integration_cli,integration_k8s \
    ./internal/integration

FROM scratch AS integration-test-darwin-arm64
COPY --link --from=integration-test-darwin-arm64-build /src/integration.test /integration-test-darwin-arm64

FROM --platform=${BUILDPLATFORM} integration-test-${TARGETOS}-${TARGETARCH} AS integration-test-targetarch

# The integration-test-provision target builds integration test binary with provisioning tests.

FROM base AS integration-test-provision-linux-build
ARG GO_BUILDFLAGS
ARG GO_LDFLAGS
ARG GOAMD64
RUN --mount=type=cache,target=/.cache,id=talos/.cache GOOS=linux GOARCH=amd64 GOAMD64=${GOAMD64} go test -v -c ${GO_BUILDFLAGS} \
    -ldflags "${GO_LDFLAGS}" \
    -tags integration,integration_provision \
    ./internal/integration

FROM scratch AS integration-test-provision-linux
COPY --link --from=integration-test-provision-linux-build /src/integration.test /integration-test-provision-linux-amd64

# The lint target performs linting on the source code.
# Per-target Go lint stages run in parallel via the buildkit DAG.
# All depend on lint-go-config (gating) by bind-mounting /verified from it.
# Cache mounts: Go cache shared (concurrency-safe), lint cache locked (golangci-lint corruption protection).

FROM base AS lint-golangci-lint-custom
RUN --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared \
    go tool github.com/golangci/golangci-lint/v2/cmd/golangci-lint custom

FROM scratch AS golangci-lint-custom
COPY --link --from=lint-golangci-lint-custom /src/custom-gcl /custom-gcl

FROM base AS lint-go-config
COPY --link --from=lint-golangci-lint-custom /src/custom-gcl /usr/local/bin/custom-gcl
RUN --mount=type=bind,source=.golangci.yml,target=/src/.golangci.yml \
    --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared \
    --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked \
    GOGC=50 GOLANGCI_LINT_CACHE=/.cache/lint custom-gcl config verify --config .golangci.yml \
    && touch /verified

FROM base AS lint-go-talos
COPY --link --from=lint-golangci-lint-custom /src/custom-gcl /usr/local/bin/custom-gcl
RUN --mount=type=bind,from=lint-go-config,source=/verified,target=/tmp/.config-verified \
    --mount=type=bind,source=.golangci.yml,target=/src/.golangci.yml \
    --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared \
    --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked \
    GOGC=50 GOLANGCI_LINT_CACHE=/.cache/lint custom-gcl run --config .golangci.yml \
    && touch /verified

FROM base AS lint-go-machinery
COPY --link --from=lint-golangci-lint-custom /src/custom-gcl /usr/local/bin/custom-gcl
WORKDIR /src/pkg/machinery
RUN --mount=type=bind,from=lint-go-config,source=/verified,target=/tmp/.config-verified \
    --mount=type=bind,source=.golangci.yml,target=/src/.golangci.yml \
    --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared \
    --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked \
    GOGC=50 GOLANGCI_LINT_CACHE=/.cache/lint custom-gcl run --config ../../.golangci.yml \
    && touch /verified

FROM base AS lint-go-tools
COPY --link --from=lint-golangci-lint-custom /src/custom-gcl /usr/local/bin/custom-gcl
RUN --mount=type=bind,from=lint-go-config,source=/verified,target=/tmp/.config-verified \
    --mount=type=bind,source=.golangci.yml,target=/src/.golangci.yml \
    --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared \
    --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked <<EOF
set -euo pipefail
for d in /src/tools/*/; do
    [ -f "$d/go.mod" ] || continue
    echo "::: linting $d"
    cd "$d"
    GOGC=50 GOLANGCI_LINT_CACHE=/.cache/lint custom-gcl run --config /src/.golangci.yml
done
touch /verified
EOF

FROM base AS lint-go-importvet
RUN --mount=type=bind,from=lint-go-config,source=/verified,target=/tmp/.config-verified \
    --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared \
    go tool github.com/siderolabs/importvet/cmd/importvet github.com/siderolabs/talos/... \
    && touch /verified

FROM scratch AS lint-go
COPY --link --from=lint-go-talos /verified /lint-go-talos
COPY --link --from=lint-go-machinery /verified /lint-go-machinery
COPY --link --from=lint-go-tools /verified /lint-go-tools
COPY --link --from=lint-go-importvet /verified /lint-go-importvet

# The lint-golangci-lint-fmt target runs the golangci-lint formatter and fixes issues automatically.
FROM base AS lint-golangci-lint-fmt-run
COPY --link --from=lint-golangci-lint-custom /src/custom-gcl /usr/local/bin/custom-gcl
COPY .golangci.yml .
ENV GOGC=50
ENV GOLANGCI_LINT_CACHE=/.cache/lint
RUN --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked custom-gcl fmt --config .golangci.yml
RUN --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked custom-gcl run --fix --issues-exit-code 0 --config .golangci.yml
WORKDIR /src/pkg/machinery
RUN --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked custom-gcl fmt --config ../../.golangci.yml
RUN --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked custom-gcl run --fix --issues-exit-code 0 --config ../../.golangci.yml
RUN --mount=type=cache,target=/.cache,id=talos/.cache,sharing=shared \
    --mount=type=cache,target=/.cache/lint,id=talos/.cache/lint,sharing=locked <<EOF
set -euo pipefail
for d in /src/tools/*/; do
    [ -f "$d/go.mod" ] || continue
    cd "$d"
    custom-gcl fmt --config /src/.golangci.yml
    custom-gcl run --fix --issues-exit-code 0 --config /src/.golangci.yml
done
EOF
WORKDIR /src

# clean golangci-lint fmt output
# exclude files populated by the `base` stage (gendata build args, generated os-release)
# so running this target doesn't dirty the source tree with build-time values.
FROM scratch AS lint-golangci-lint-fmt
COPY --link --from=lint-golangci-lint-fmt-run \
    --exclude=pkg/machinery/gendata/data \
    --exclude=pkg/machinery/version/os-release \
    /src .

# The protolint target performs linting on protobuf files.

FROM base AS lint-protobuf
COPY --link --from=api-descriptors /api/lock.binpb /tmp/current.lock.binpb
WORKDIR /src/api
RUN --mount=type=bind,source=api,target=/src/api --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/bufbuild/buf/cmd/buf lint
RUN --mount=type=bind,source=api,target=/src/api --mount=type=cache,target=/.cache,id=talos/.cache go tool github.com/bufbuild/buf/cmd/buf breaking /tmp/current.lock.binpb --against lock.binpb

# The markdownlint target performs linting on Markdown files.

FROM oven/bun:1-alpine AS lint-markdown
ARG MARKDOWNLINTCLI_VERSION
RUN apk add --no-cache findutils
RUN bun i -g markdownlint-cli@${MARKDOWNLINTCLI_VERSION}
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

# The docs target generates documentation.

FROM base AS docs-build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
RUN --mount=type=bind,from=talosctl-targetarch,source=/talosctl-${TARGETOS}-${TARGETARCH},target=/bin/talosctl \
    env HOME=/home/user TAG=latest /bin/talosctl docs --config /tmp/configuration \
    && env HOME=/home/user TAG=latest /bin/talosctl docs --cli /tmp
COPY ./pkg/machinery/config/schemas/*.schema.json /tmp/schemas/

FROM scratch AS docs
COPY --link --from=docs-build /tmp/configuration/ /website/content/v1.14/reference/configuration/
COPY --link --from=docs-build /tmp/cli.md /website/content/v1.14/reference/
COPY --link --from=docs-build /tmp/schemas /website/content/v1.14/schemas/

# The talosctl-cni-bundle builds the CNI bundle for talosctl.

FROM scratch AS talosctl-cni-bundle
ARG TARGETARCH
COPY --link --from=pkgs-talosctl-cni-bundle /opt/cni/bin/ /talosctl-cni-bundle-${TARGETARCH}/
