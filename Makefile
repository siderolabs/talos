REGISTRY ?= ghcr.io
USERNAME ?= siderolabs
SHA ?= $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG ?= $(shell git describe --tag --always --dirty --match v[0-9]\*)
ABBREV_TAG ?= $(shell git describe --tag --always --match v[0-9]\* --abbrev=0 )
TAG_SUFFIX ?=
SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct)
IMAGE_REGISTRY ?= $(REGISTRY)
IMAGE_TAG ?= $(TAG)$(TAG_SUFFIX)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
REGISTRY_AND_USERNAME := $(IMAGE_REGISTRY)/$(USERNAME)
NAME = Talos

CLOUD_IMAGES_EXTRA_ARGS ?= ""
ZSTD_COMPRESSION_LEVEL ?= 18

CI_RELEASE_TAG := $(shell git log --oneline --format=%B -n 1 HEAD^2 -- 2>/dev/null | head -n 1 | sed -r "/^release\(.*\)/ s/^release\((.*)\):.*$$/\\1/; t; Q")

ARTIFACTS := _out
TOOLS ?= ghcr.io/siderolabs/tools:v1.10.0-alpha.0-6-gbc30a2a

DEBUG_TOOLS_SOURCE := scratch
EMBED_TARGET ?= embed

PKGS_PREFIX ?= ghcr.io/siderolabs
PKGS ?= v1.10.0-alpha.0-26-gcfb4b0a
EXTRAS ?= v1.9.0

KRES_IMAGE ?= ghcr.io/siderolabs/kres:latest
CONFORMANCE_IMAGE ?= ghcr.io/siderolabs/conform:latest

PKG_FHS ?= $(PKGS_PREFIX)/fhs:$(PKGS)
PKG_CA_CERTIFICATES ?= $(PKGS_PREFIX)/ca-certificates:$(PKGS)
PKG_APPARMOR ?= $(PKGS_PREFIX)/apparmor:$(PKGS)
PKG_CRYPTSETUP ?= $(PKGS_PREFIX)/cryptsetup:$(PKGS)
PKG_CONTAINERD ?= $(PKGS_PREFIX)/containerd:$(PKGS)
PKG_DOSFSTOOLS ?= $(PKGS_PREFIX)/dosfstools:$(PKGS)
PKG_E2FSPROGS ?= $(PKGS_PREFIX)/e2fsprogs:$(PKGS)
PKG_SYSTEMD_UDEVD ?= $(PKGS_PREFIX)/systemd-udevd:$(PKGS)
PKG_LIBCAP ?= $(PKGS_PREFIX)/libcap:$(PKGS)
PKG_GRUB ?= $(PKGS_PREFIX)/grub:$(PKGS)
PKG_SD_BOOT ?= $(PKGS_PREFIX)/sd-boot:$(PKGS)
PKG_IPTABLES ?= $(PKGS_PREFIX)/iptables:$(PKGS)
PKG_IPXE ?= $(PKGS_PREFIX)/ipxe:$(PKGS)
PKG_LIBINIH ?= $(PKGS_PREFIX)/libinih:$(PKGS)
PKG_LIBJSON_C ?= $(PKGS_PREFIX)/libjson-c:$(PKGS)
PKG_LIBMNL ?= $(PKGS_PREFIX)/libmnl:$(PKGS)
PKG_LIBNFTNL ?= $(PKGS_PREFIX)/libnftnl:$(PKGS)
PKG_LIBPOPT ?= $(PKGS_PREFIX)/libpopt:$(PKGS)
PKG_LIBSEPOL ?= $(PKGS_PREFIX)/libsepol:$(PKGS)
PKG_LIBSELINUX ?= $(PKGS_PREFIX)/libselinux:$(PKGS)
PKG_PCRE2 ?= $(PKGS_PREFIX)/pcre2:$(PKGS)
PKG_LIBURCU ?= $(PKGS_PREFIX)/liburcu:$(PKGS)
PKG_OPENSSL ?= $(PKGS_PREFIX)/openssl:$(PKGS)
PKG_LIBSECCOMP ?= $(PKGS_PREFIX)/libseccomp:$(PKGS)
PKG_LINUX_FIRMWARE ?= $(PKGS_PREFIX)/linux-firmware:$(PKGS)
PKG_LVM2 ?= $(PKGS_PREFIX)/lvm2:$(PKGS)
PKG_LIBAIO ?= $(PKGS_PREFIX)/libaio:$(PKGS)
PKG_MUSL ?= $(PKGS_PREFIX)/musl:$(PKGS)
PKG_RUNC ?= $(PKGS_PREFIX)/runc:$(PKGS)
PKG_XFSPROGS ?= $(PKGS_PREFIX)/xfsprogs:$(PKGS)
PKG_UTIL_LINUX ?= $(PKGS_PREFIX)/util-linux:$(PKGS)
PKG_KMOD ?= $(PKGS_PREFIX)/kmod:$(PKGS)
PKG_CNI ?= $(PKGS_PREFIX)/cni:$(PKGS)
PKG_FLANNEL_CNI ?= $(PKGS_PREFIX)/flannel-cni:$(PKGS)
PKG_KERNEL ?= $(PKGS_PREFIX)/kernel:$(PKGS)
PKG_TALOSCTL_CNI_BUNDLE_INSTALL ?= $(PKGS_PREFIX)/talosctl-cni-bundle-install:$(EXTRAS)

# renovate: datasource=github-tags depName=golang/go
GO_VERSION ?= 1.23
# renovate: datasource=go depName=golang.org/x/tools
GOIMPORTS_VERSION ?= v0.28.0
# renovate: datasource=go depName=mvdan.cc/gofumpt
GOFUMPT_VERSION ?= v0.7.0
# renovate: datasource=go depName=github.com/golangci/golangci-lint
GOLANGCILINT_VERSION ?= v1.62.2
# renovate: datasource=go depName=golang.org/x/tools
STRINGER_VERSION ?= v0.28.0
# renovate: datasource=go depName=github.com/dmarkham/enumer
ENUMER_VERSION ?= v1.5.10
# renovate: datasource=go depName=k8s.io/code-generator
DEEPCOPY_GEN_VERSION ?= v0.32.0
# renovate: datasource=go depName=github.com/planetscale/vtprotobuf
VTPROTOBUF_VERSION ?= v0.6.0
# renovate: datasource=go depName=github.com/siderolabs/deep-copy
DEEPCOPY_VERSION ?= v0.5.6
# renovate: datasource=go depName=github.com/siderolabs/importvet
IMPORTVET_VERSION ?= v0.2.0
# not setting renovate config since the repo is archived
PROTOTOOL_VERSION ?= v1.10.0
# renovate: datasource=go depName=github.com/pseudomuto/protoc-gen-doc
PROTOC_GEN_DOC_VERSION ?= v1.5.1
# renovate: datasource=npm depName=markdownlint-cli
MARKDOWNLINTCLI_VERSION ?= 0.43.0
# renovate: datasource=npm depName=textlint
TEXTLINT_VERSION ?= 14.4.0
# renovate: datasource=npm depName=textlint-filter-rule-comments
TEXTLINT_FILTER_RULE_COMMENTS_VERSION ?= 1.2.2
# renovate: datasource=npm depName=textlint-rule-one-sentence-per-line
TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION ?= 2.0.0
# renovate: datasource=docker depName=klakegg/hugo
HUGO_VERSION ?= 0.111.3-ext-alpine
OPERATING_SYSTEM := $(shell uname -s | tr "[:upper:]" "[:lower:]")
ARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
TALOSCTL_DEFAULT_TARGET := talosctl-$(OPERATING_SYSTEM)
TALOSCTL_EXECUTABLE := $(PWD)/$(ARTIFACTS)/$(TALOSCTL_DEFAULT_TARGET)-$(ARCH)
INTEGRATION_TEST := integration-test
INTEGRATION_TEST_DEFAULT_TARGET := $(INTEGRATION_TEST)-$(OPERATING_SYSTEM)
INTEGRATION_TEST_PROVISION_DEFAULT_TARGET := integration-test-provision-$(OPERATING_SYSTEM)
# renovate: datasource=github-releases depName=kubernetes/kubernetes
KUBECTL_VERSION ?= v1.32.1
# renovate: datasource=github-releases depName=kastenhq/kubestr
KUBESTR_VERSION ?= v0.4.47
# renovate: datasource=github-releases depName=helm/helm
HELM_VERSION ?= v3.16.4
# renovate: datasource=github-releases depName=cilium/cilium-cli
CILIUM_CLI_VERSION ?= v0.16.22
# renovate: datasource=github-releases depName=microsoft/secureboot_objects
MICROSOFT_SECUREBOOT_RELEASE ?= v1.1.3

KUBECTL_URL ?= https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(OPERATING_SYSTEM)/amd64/kubectl
KUBESTR_URL ?= https://github.com/kastenhq/kubestr/releases/download/$(KUBESTR_VERSION)/kubestr_$(subst v,,$(KUBESTR_VERSION))_Linux_amd64.tar.gz
HELM_URL ?= https://get.helm.sh/helm-$(HELM_VERSION)-linux-amd64.tar.gz
CILIUM_CLI_URL ?= https://github.com/cilium/cilium-cli/releases/download/$(CILIUM_CLI_VERSION)/cilium-$(OPERATING_SYSTEM)-amd64.tar.gz
TESTPKGS ?= github.com/siderolabs/talos/...
RELEASES ?= v1.8.4 v1.9.0
SHORT_INTEGRATION_TEST ?=
CUSTOM_CNI_URL ?=

INSTALLER_ARCH ?= all
INSTALLER_ONLY_PKGS ?= \
    bash \
    cpio \
    efibootmgr \
    kmod \
    squashfs-tools \
    xfsprogs \
    xz \
    zstd

IMAGER_EXTRA_PKGS ?= \
    dosfstools \
    e2fsprogs \
    mtools \
    pigz \
    qemu-img \
    tar \
    xorriso

INSTALLER_PKGS ?= $(INSTALLER_ONLY_PKGS) $(IMAGER_EXTRA_PKGS)
IMAGER_ARGS ?=

CGO_ENABLED ?= 0
GO_BUILDFLAGS ?=
GO_BUILDTAGS ?= tcell_minimal,grpcnotrace
GO_BUILDTAGS_TALOSCTL ?= grpcnotrace
GO_LDFLAGS ?=
GOAMD64 ?= v2

WITH_RACE ?= false
WITH_DEBUG ?= false

ifneq (, $(filter $(WITH_RACE), t true TRUE y yes 1))
CGO_ENABLED = 1
GO_BUILDFLAGS += -race
GO_LDFLAGS += -linkmode=external -extldflags '-static'
INSTALLER_ARCH = targetarch
endif

ifneq (, $(filter $(WITH_DEBUG), t true TRUE y yes 1))
GO_BUILDTAGS := $(GO_BUILDTAGS),sidero.debug
GO_BUILDTAGS_TALOSCTL := $(GO_BUILDTAGS_TALOSCTL),sidero.debug
else
GO_LDFLAGS += -s -w
endif

ifneq (, $(filter $(WITH_DEBUG_SHELL), t true TRUE y yes 1))
# bash-minimal is a Dockerfile target that copies over the bash from siderolabs tools
DEBUG_TOOLS_SOURCE := bash-minimal
endif

GO_BUILDFLAGS_TALOSCTL := $(GO_BUILDFLAGS) -tags "$(GO_BUILDTAGS_TALOSCTL)"
GO_BUILDFLAGS += -tags "$(GO_BUILDTAGS)"

, := ,
space := $(subst ,, )
BUILD := docker buildx build
PLATFORM ?= linux/amd64
PROGRESS ?= auto
PUSH ?= false
COMMON_ARGS := --file=Dockerfile
COMMON_ARGS += --progress=$(PROGRESS)
COMMON_ARGS += --platform=$(PLATFORM)
COMMON_ARGS += --push=$(PUSH)
COMMON_ARGS += --build-arg=TOOLS=$(TOOLS)
COMMON_ARGS += --build-arg=DEBUG_TOOLS_SOURCE=$(DEBUG_TOOLS_SOURCE)
COMMON_ARGS += --build-arg=PKGS=$(PKGS)
COMMON_ARGS += --build-arg=EXTRAS=$(EXTRAS)
COMMON_ARGS += --build-arg=EMBED_TARGET=$(EMBED_TARGET)
COMMON_ARGS += --build-arg=GOFUMPT_VERSION=$(GOFUMPT_VERSION)
COMMON_ARGS += --build-arg=GOIMPORTS_VERSION=$(GOIMPORTS_VERSION)
COMMON_ARGS += --build-arg=STRINGER_VERSION=$(STRINGER_VERSION)
COMMON_ARGS += --build-arg=ENUMER_VERSION=$(ENUMER_VERSION)
COMMON_ARGS += --build-arg=DEEPCOPY_GEN_VERSION=$(DEEPCOPY_GEN_VERSION)
COMMON_ARGS += --build-arg=VTPROTOBUF_VERSION=$(VTPROTOBUF_VERSION)
COMMON_ARGS += --build-arg=IMPORTVET_VERSION=$(IMPORTVET_VERSION)
COMMON_ARGS += --build-arg=PROTOTOOL_VERSION=$(PROTOTOOL_VERSION)
COMMON_ARGS += --build-arg=PROTOC_GEN_DOC_VERSION=$(PROTOC_GEN_DOC_VERSION)
COMMON_ARGS += --build-arg=GOLANGCILINT_VERSION=$(GOLANGCILINT_VERSION)
COMMON_ARGS += --build-arg=DEEPCOPY_VERSION=$(DEEPCOPY_VERSION)
COMMON_ARGS += --build-arg=MARKDOWNLINTCLI_VERSION=$(MARKDOWNLINTCLI_VERSION)
COMMON_ARGS += --build-arg=TEXTLINT_VERSION=$(TEXTLINT_VERSION)
COMMON_ARGS += --build-arg=TEXTLINT_FILTER_RULE_COMMENTS_VERSION=$(TEXTLINT_FILTER_RULE_COMMENTS_VERSION)
COMMON_ARGS += --build-arg=TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION=$(TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION)
COMMON_ARGS += --build-arg=TAG=$(TAG)
COMMON_ARGS += --build-arg=SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH)
COMMON_ARGS += --build-arg=ARTIFACTS=$(ARTIFACTS)
COMMON_ARGS += --build-arg=TESTPKGS=$(TESTPKGS)
COMMON_ARGS += --build-arg=INSTALLER_ARCH=$(INSTALLER_ARCH)
COMMON_ARGS += --build-arg=INSTALLER_PKGS="$(INSTALLER_PKGS)"
COMMON_ARGS += --build-arg=CGO_ENABLED=$(CGO_ENABLED)
COMMON_ARGS += --build-arg=GO_BUILDFLAGS="$(GO_BUILDFLAGS)"
COMMON_ARGS += --build-arg=GO_BUILDFLAGS_TALOSCTL="$(GO_BUILDFLAGS_TALOSCTL)"
COMMON_ARGS += --build-arg=GO_LDFLAGS="$(GO_LDFLAGS)"
COMMON_ARGS += --build-arg=GOAMD64="$(GOAMD64)"
COMMON_ARGS += --build-arg=http_proxy=$(http_proxy)
COMMON_ARGS += --build-arg=https_proxy=$(https_proxy)
COMMON_ARGS += --build-arg=NAME=$(NAME)
COMMON_ARGS += --build-arg=SHA=$(SHA)
COMMON_ARGS += --build-arg=USERNAME=$(USERNAME)
COMMON_ARGS += --build-arg=REGISTRY=$(REGISTRY)
COMMON_ARGS += --build-arg=PKGS_PREFIX=$(PKGS_PREFIX)
COMMON_ARGS += --build-arg=PKG_FHS=$(PKG_FHS)
COMMON_ARGS += --build-arg=PKG_CA_CERTIFICATES=$(PKG_CA_CERTIFICATES)
COMMON_ARGS += --build-arg=PKG_APPARMOR=$(PKG_APPARMOR)
COMMON_ARGS += --build-arg=PKG_CRYPTSETUP=$(PKG_CRYPTSETUP)
COMMON_ARGS += --build-arg=PKG_CONTAINERD=$(PKG_CONTAINERD)
COMMON_ARGS += --build-arg=PKG_DOSFSTOOLS=$(PKG_DOSFSTOOLS)
COMMON_ARGS += --build-arg=PKG_E2FSPROGS=$(PKG_E2FSPROGS)
COMMON_ARGS += --build-arg=PKG_SYSTEMD_UDEVD=$(PKG_SYSTEMD_UDEVD)
COMMON_ARGS += --build-arg=PKG_LIBCAP=$(PKG_LIBCAP)
COMMON_ARGS += --build-arg=PKG_GRUB=$(PKG_GRUB)
COMMON_ARGS += --build-arg=PKG_SD_BOOT=$(PKG_SD_BOOT)
COMMON_ARGS += --build-arg=PKG_IPTABLES=$(PKG_IPTABLES)
COMMON_ARGS += --build-arg=PKG_IPXE=$(PKG_IPXE)
COMMON_ARGS += --build-arg=PKG_LIBINIH=$(PKG_LIBINIH)
COMMON_ARGS += --build-arg=PKG_LIBJSON_C=$(PKG_LIBJSON_C)
COMMON_ARGS += --build-arg=PKG_LIBMNL=$(PKG_LIBMNL)
COMMON_ARGS += --build-arg=PKG_LIBNFTNL=$(PKG_LIBNFTNL)
COMMON_ARGS += --build-arg=PKG_LIBSEPOL=$(PKG_LIBSEPOL)
COMMON_ARGS += --build-arg=PKG_LIBSELINUX=$(PKG_LIBSELINUX)
COMMON_ARGS += --build-arg=PKG_PCRE2=$(PKG_PCRE2)
COMMON_ARGS += --build-arg=PKG_LIBPOPT=$(PKG_LIBPOPT)
COMMON_ARGS += --build-arg=PKG_LIBURCU=$(PKG_LIBURCU)
COMMON_ARGS += --build-arg=PKG_OPENSSL=$(PKG_OPENSSL)
COMMON_ARGS += --build-arg=PKG_LIBSECCOMP=$(PKG_LIBSECCOMP)
COMMON_ARGS += --build-arg=PKG_LINUX_FIRMWARE=$(PKG_LINUX_FIRMWARE)
COMMON_ARGS += --build-arg=PKG_LVM2=$(PKG_LVM2)
COMMON_ARGS += --build-arg=PKG_LIBAIO=$(PKG_LIBAIO)
COMMON_ARGS += --build-arg=PKG_MUSL=$(PKG_MUSL)
COMMON_ARGS += --build-arg=PKG_RUNC=$(PKG_RUNC)
COMMON_ARGS += --build-arg=PKG_XFSPROGS=$(PKG_XFSPROGS)
COMMON_ARGS += --build-arg=PKG_UTIL_LINUX=$(PKG_UTIL_LINUX)
COMMON_ARGS += --build-arg=PKG_KMOD=$(PKG_KMOD)
COMMON_ARGS += --build-arg=PKG_U_BOOT=$(PKG_U_BOOT)
COMMON_ARGS += --build-arg=PKG_RASPBERYPI_FIRMWARE=$(PKG_RASPBERYPI_FIRMWARE)
COMMON_ARGS += --build-arg=PKG_CNI=$(PKG_CNI)
COMMON_ARGS += --build-arg=PKG_FLANNEL_CNI=$(PKG_FLANNEL_CNI)
COMMON_ARGS += --build-arg=PKG_KERNEL=$(PKG_KERNEL)
COMMON_ARGS += --build-arg=PKG_TALOSCTL_CNI_BUNDLE_INSTALL=$(PKG_TALOSCTL_CNI_BUNDLE_INSTALL)
COMMON_ARGS += --build-arg=ABBREV_TAG=$(ABBREV_TAG)
COMMON_ARGS += --build-arg=ZSTD_COMPRESSION_LEVEL=$(ZSTD_COMPRESSION_LEVEL)
COMMON_ARGS += --build-arg=MICROSOFT_SECUREBOOT_RELEASE=$(MICROSOFT_SECUREBOOT_RELEASE)

CI_ARGS ?=

EXTENSIONS_FILTER_COMMAND ?= grep -vE 'tailscale|xen-guest-agent|nvidia|vmtoolsd-guest-agent|metal-agent|cloudflared'

all: initramfs kernel installer imager talosctl talosctl-image talos

# Help Menu

define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- docker (19.03 or higher)
- buildx (https://github.com/docker/buildx)
- crane (https://github.com/google/go-containerregistry/blob/main/cmd/crane/README.md)

## Creating a Builder Instance

The build process makes use of features not currently supported by the default
builder instance (docker driver). To create a compatible builder instance, run:

```
docker buildx create --driver docker-container --name local --buildkitd-flags '--allow-insecure-entitlement security.insecure' --use
```

If you already have a compatible builder instance, you may use that instead.

> Note: The security.insecure entitlement is only required, and used by the unit-tests target.

## Artifacts

All artifacts will be output to ./$(ARTIFACTS). Images will be tagged with the
registry "$(IMAGE_REGISTRY)", username "$(USERNAME)", and a dynamic tag (e.g. $(REGISTRY_AND_USERNAME)/image:$(IMAGE_TAG)).
The registry and username can be overridden by exporting REGISTRY, and USERNAME
respectively.

## Race Detector

Building with `WITH_RACE=1` enables race detector in the Talos executables. Integration tests are always built with the race detector
enabled.

endef

export HELP_MENU_HEADER

help: ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z0-9%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Build Abstractions

.PHONY: base
target-%: ## Builds the specified target defined in the Dockerfile. The build result will only remain in the build cache.
	@$(BUILD) \
		--target=$* \
		$(COMMON_ARGS) \
		$(TARGET_ARGS) \
		$(CI_ARGS) .

local-%: ## Builds the specified target defined in the Dockerfile using the local output type. The build result will be output to the specified local destination.
	@$(MAKE) target-$* TARGET_ARGS="--output=type=local,dest=$(DEST) $(TARGET_ARGS)"
	@PLATFORM=$(PLATFORM) \
		ARTIFACTS=$(ARTIFACTS) \
		./hack/fix-artifacts.sh

docker-%: ## Builds the specified target defined in the Dockerfile using the docker output type. The build result will be output to the specified local destination.
	@mkdir -p $(DEST)
	@$(MAKE) target-$* TARGET_ARGS="--output type=docker,dest=$(DEST)/$*.tar,name=$(REGISTRY_AND_USERNAME)/$*:$(IMAGE_TAG) $(TARGET_ARGS)"

registry-%: ## Builds the specified target defined in the Dockerfile using the image/registry output type. The build result will be pushed to the registry if PUSH=true.
	@$(MAKE) target-$* TARGET_ARGS="--output type=image,name=$(REGISTRY_AND_USERNAME)/$*:$(IMAGE_TAG) $(TARGET_ARGS)"

hack-test-%: ## Runs the specified script in ./hack/test with well known environment variables.
	@./hack/test/$*.sh

# Generators

.PHONY: generate
generate: ## Generates code from protobuf service definitions and machinery config.
	@$(MAKE) local-$@ DEST=./ PLATFORM=linux/amd64 EMBED_TARGET=embed-abbrev

.PHONY: docs
docs: ## Generates the documentation for machine config, and talosctl.
	@rm -rf docs/configuration/*
	@rm -rf docs/talosctl/*
	@$(MAKE) local-$@ DEST=./ PLATFORM=linux/amd64

.PHONY: docs-preview
docs-preview: ## Starts a local preview of the documentation using Hugo in docker
	@docker run --rm --interactive --tty \
	--user $(shell id -u):$(shell id -g) \
	--volume $(PWD):/src --workdir /src/website \
	--publish 1313:1313 \
	klakegg/hugo:$(HUGO_VERSION) \
	server

# Local Artifacts

.PHONY: kernel
kernel: ## Outputs the kernel package contents (vmlinuz) to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false
	@-rm -rf $(ARTIFACTS)/modules

.PHONY: initramfs
initramfs: ## Builds the compressed initramfs and outputs it to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false

.PHONY: sd-boot
sd-boot: ## Outputs the systemd-boot to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false

.PHONY: sd-stub
sd-stub: ## Outputs the systemd-stub to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false

.PHONY: installer
installer: ## Builds the container image for the installer and outputs it to the registry.
	@INSTALLER_ARCH=targetarch INSTALLER_PKGS="$(INSTALLER_ONLY_PKGS)" \
		$(MAKE) registry-$@

.PHONY: imager
imager: ## Builds the container image for the imager and outputs it to the registry.
	@$(MAKE) registry-$@

.PHONY: talos
talos: ## Builds the Talos container image and outputs it to the registry.
	@$(MAKE) registry-$@

.PHONY: talosctl-image
talosctl-image: ## Builds the talosctl container image and outputs it to the registry.
	@$(MAKE) registry-talosctl

talosctl-all:
	@$(MAKE) local-talosctl-all DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl-linux-amd64:
	@$(MAKE) local-talosctl-linux-amd64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl-linux-arm64:
	@$(MAKE) local-talosctl-linux-arm64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl-darwin-amd64:
	@$(MAKE) local-talosctl-darwin-amd64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl-darwin-arm64:
	@$(MAKE) local-talosctl-darwin-arm64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl-freebsd-amd64:
	@$(MAKE) local-talosctl-freebsd-amd64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

taloscl-freebsd-arm64:
	@$(MAKE) local-talosctl-freebsd-arm64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl-windows-amd64:
	@$(MAKE) local-talosctl-windows-amd64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl-windows-arm64:
	@$(MAKE) local-talosctl-windows-arm64 DEST=$(ARTIFACTS) PUSH=false NAME=Client

talosctl:
	@$(MAKE) local-talosctl-targetarch DEST=$(ARTIFACTS)

image-%: ## Builds the specified image. Valid options are aws, azure, digital-ocean, gcp, and vmware (e.g. image-aws)
	@docker pull $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG)
	@for platform in $(subst $(,),$(space),$(PLATFORM)); do \
		arch=$$(basename "$${platform}") && \
		docker run --rm -t -v /dev:/dev -v $(PWD)/$(ARTIFACTS):/secureboot:ro -v $(PWD)/$(ARTIFACTS):/out -e SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) --network=host --privileged $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG) $* --arch $$arch $(IMAGER_ARGS) ; \
	done

images-essential: image-aws image-azure image-gcp image-metal secureboot-installer ## Builds only essential images used in the CI (AWS, GCP, and Metal).

images: image-akamai image-aws image-azure image-digital-ocean image-exoscale image-cloudstack image-gcp image-hcloud image-iso image-metal image-nocloud image-opennebula image-openstack image-oracle image-scaleway image-upcloud image-vmware image-vultr ## Builds all known images (AWS, Azure, DigitalOcean, Exoscale, Cloudstack, GCP, HCloud, Metal, NoCloud, OpenNebula, OpenStack, Oracle, Scaleway, UpCloud, Vultr and VMware).

.PHONY: iso
iso: image-iso ## Builds the ISO and outputs it to the artifact directory.

.PHONY: secureboot-iso
secureboot-iso: image-secureboot-iso ## Builds UEFI only ISO which uses UKI and outputs it to the artifact directory.

.PHONY: secureboot-installer
secureboot-installer: ## Builds UEFI only installer which uses UKI and push it to the registry.
	@$(MAKE) image-secureboot-installer IMAGER_ARGS="--base-installer-image $(REGISTRY_AND_USERNAME)/installer:$(IMAGE_TAG) $(IMAGER_ARGS)"
	@for platform in $(subst $(,),$(space),$(PLATFORM)); do \
		arch=$$(basename "$${platform}") && \
		crane push $(ARTIFACTS)/installer-$${arch}-secureboot.tar $(REGISTRY_AND_USERNAME)/installer:$(IMAGE_TAG)-$${arch}-secureboot ; \
	done

.PHONY: talosctl-cni-bundle
talosctl-cni-bundle: ## Creates a compressed tarball that includes CNI bundle for talosctl.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)
	@for platform in $(subst $(,),$(space),$(PLATFORM)); do \
		arch=`basename "$${platform}"` ; \
		tar  -C $(ARTIFACTS)/talosctl-cni-bundle-$${arch} -czf $(ARTIFACTS)/talosctl-cni-bundle-$${arch}.tar.gz . ; \
	done
	@rm -rf $(ARTIFACTS)/talosctl-cni-bundle-*/

.PHONY: cloud-images
cloud-images: ## Uploads cloud images (AMIs, etc.) to the cloud registry.
	@docker run --rm -v $(PWD):/src -w /src \
		-e TAG=$(TAG) -e ARTIFACTS=$(ARTIFACTS) -e ABBREV_TAG=$(ABBREV_TAG) \
		-e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY \
		-e AZURE_SUBSCRIPTION_ID -e AZURE_CLIENT_ID -e AZURE_CLIENT_SECRET -e AZURE_TENANT_ID \
		-e GOOGLE_PROJECT_ID -e GOOGLE_CREDENTIALS \
		golang:$(GO_VERSION) \
		./hack/cloud-image-uploader.sh $(CLOUD_IMAGES_EXTRA_ARGS)

.PHONY: uki-certs
uki-certs: talosctl ## Generate test certificates for SecureBoot/PCR Signing
	@$(TALOSCTL_EXECUTABLE) gen secureboot uki
	@$(TALOSCTL_EXECUTABLE) gen secureboot pcr
	@$(TALOSCTL_EXECUTABLE) gen secureboot database

.PHONY: cache-create
cache-create: installer imager ## Generate image cache.
	@docker run --entrypoint /usr/local/bin/e2e.test registry.k8s.io/conformance:$(KUBECTL_VERSION) --list-images | \
		$(TALOSCTL_EXECUTABLE) images integration --installer-tag=$(IMAGE_TAG) --registry-and-user=$(REGISTRY_AND_USERNAME) | \
		$(TALOSCTL_EXECUTABLE) images cache-create --image-cache-path=/tmp/cache.tar --images=- --force
	@crane push /tmp/cache.tar $(REGISTRY_AND_USERNAME)/image-cache:$(IMAGE_TAG)
	@$(MAKE) image-iso IMAGER_ARGS="--image-cache=$(REGISTRY_AND_USERNAME)/image-cache:$(IMAGE_TAG) --extra-kernel-arg='console=ttyS0'"

# Code Quality

api-descriptors: ## Generates API descriptors used to detect breaking API changes.
	@$(MAKE) local-api-descriptors DEST=./ PLATFORM=linux/amd64

fmt-go: ## Formats the source code.
	@docker run --rm -it -v $(PWD):/src -w /src -e GOTOOLCHAIN=local golang:$(GO_VERSION) bash -c "go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION) && goimports -w -local github.com/siderolabs/talos . && go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION) && gofumpt -w ."

fmt-protobuf: ## Formats protobuf files.
	@$(MAKE) local-fmt-protobuf DEST=./ PLATFORM=linux/amd64

fmt: ## Formats the source code and protobuf files.
	@$(MAKE) fmt-go fmt-protobuf

lint-%: ## Runs the specified linter. Valid options are go, protobuf, and markdown (e.g. lint-go).
	@$(MAKE) target-lint-$* PLATFORM=linux/amd64

lint: ## Runs linters on go, vulncheck, protobuf, and markdown file types.
	@$(MAKE) lint-go lint-vulncheck lint-protobuf lint-markdown

check-dirty: ## Verifies that source tree is not dirty
	@if test -n "`git status --porcelain`"; then echo "Source tree is dirty"; git status; git diff; exit 1 ; fi

go-mod-outdated: ## Runs the go-mod-oudated to show outdated dependencies.
	@$(MAKE) target-go-mod-outdated PLATFORM=linux/amd64

# Tests

.PHONY: unit-tests
unit-tests: ## Performs unit tests.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) TARGET_ARGS="--allow security.insecure" PLATFORM=linux/amd64

.PHONY: unit-tests-race
unit-tests-race: ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@ TARGET_ARGS="--allow security.insecure" PLATFORM=linux/amd64

$(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64 DEST=$(ARTIFACTS) PLATFORM=linux/amd64 WITH_RACE=true NAME=Client PUSH=false

$(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-arm64:
	@$(MAKE) local-$(INTEGRATION_TEST_DEFAULT_TARGET)-arm64 DEST=$(ARTIFACTS) PLATFORM=linux/arm64 WITH_RACE=true NAME=Client PUSH=false

$(ARTIFACTS)/$(INTEGRATION_TEST):
	@$(MAKE) local-$(INTEGRATION_TEST)-targetarch DEST=$(ARTIFACTS)

$(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET) DEST=$(ARTIFACTS) PLATFORM=linux/amd64 WITH_RACE=true NAME=Client

$(ARTIFACTS)/kubectl:
	@mkdir -p $(ARTIFACTS)
	@curl -L -o $(ARTIFACTS)/kubectl "$(KUBECTL_URL)"
	@chmod +x $(ARTIFACTS)/kubectl

$(ARTIFACTS)/kubestr:
	@mkdir -p $(ARTIFACTS)
	@curl -L "$(KUBESTR_URL)" | tar xzf - -C $(ARTIFACTS) kubestr
	@chmod +x $(ARTIFACTS)/kubestr

$(ARTIFACTS)/helm:
	@mkdir -p $(ARTIFACTS)
	@curl -L "$(HELM_URL)" | tar xzf - -C $(ARTIFACTS) --strip-components=1 linux-amd64/helm
	@chmod +x $(ARTIFACTS)/helm

$(ARTIFACTS)/cilium:
	@mkdir -p $(ARTIFACTS)
	@curl -L "$(CILIUM_CLI_URL)" | tar xzf - -C $(ARTIFACTS) cilium
	@chmod +x $(ARTIFACTS)/cilium

external-artifacts: $(ARTIFACTS)/kubectl $(ARTIFACTS)/kubestr $(ARTIFACTS)/helm $(ARTIFACTS)/cilium

e2e-%: $(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64 external-artifacts ## Runs the E2E test for the specified platform (e.g. e2e-docker).
	@$(MAKE) hack-test-$@ \
		PLATFORM=$* \
		TAG=$(TAG) \
		SHA=$(SHA) \
		REGISTRY=$(IMAGE_REGISTRY) \
		IMAGE=$(REGISTRY_AND_USERNAME)/talos:$(IMAGE_TAG) \
		INSTALLER_IMAGE=$(REGISTRY_AND_USERNAME)/installer:$(IMAGE_TAG) \
		ARTIFACTS=$(ARTIFACTS) \
		TALOSCTL=$(PWD)/$(ARTIFACTS)/$(TALOSCTL_DEFAULT_TARGET)-amd64 \
		INTEGRATION_TEST=$(PWD)/$(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64 \
		SHORT_INTEGRATION_TEST=$(SHORT_INTEGRATION_TEST) \
		CUSTOM_CNI_URL=$(CUSTOM_CNI_URL) \
		KUBECTL=$(PWD)/$(ARTIFACTS)/kubectl \
		KUBESTR=$(PWD)/$(ARTIFACTS)/kubestr \
		HELM=$(PWD)/$(ARTIFACTS)/helm \
		CILIUM_CLI=$(PWD)/$(ARTIFACTS)/cilium

provision-tests-prepare: release-artifacts $(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64

provision-tests: provision-tests-prepare
	@$(MAKE) hack-test-$@ \
		TAG=$(TAG) \
		TALOSCTL=$(PWD)/$(ARTIFACTS)/$(TALOSCTL_DEFAULT_TARGET)-amd64 \
		INTEGRATION_TEST=$(PWD)/$(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64

provision-tests-track-%:
	@$(MAKE) hack-test-provision-tests \
		TAG=$(TAG) \
		TALOSCTL=$(PWD)/$(ARTIFACTS)/$(TALOSCTL_DEFAULT_TARGET)-amd64 \
		INTEGRATION_TEST=$(PWD)/$(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64 \
		INTEGRATION_TEST_RUN="TestIntegration/.+-TR$*" \
		INTEGRATION_TEST_TRACK="$*" \
		CUSTOM_CNI_URL=$(CUSTOM_CNI_URL) \
		REGISTRY=$(IMAGE_REGISTRY) \
		ARTIFACTS=$(ARTIFACTS)

installer-with-extensions: $(ARTIFACTS)/extensions/_out/extensions-metadata
	$(MAKE) image-installer \
		IMAGER_ARGS="--base-installer-image=$(REGISTRY_AND_USERNAME)/installer:$(IMAGE_TAG) $(shell cat $(ARTIFACTS)/extensions/_out/extensions-metadata | $(EXTENSIONS_FILTER_COMMAND) | xargs -n 1 echo --system-extension-image)"
	crane push $(ARTIFACTS)/installer-amd64.tar $(REGISTRY_AND_USERNAME)/installer:$(IMAGE_TAG)-amd64-extensions
	INSTALLER_IMAGE_EXTENSIONS="$(REGISTRY_AND_USERNAME)/installer:$(IMAGE_TAG)-amd64-extensions" yq eval -n '.machine.install.image = strenv(INSTALLER_IMAGE_EXTENSIONS)' > $(ARTIFACTS)/installer-extensions-patch.yaml

kubelet-fat-patch:
	K8S_VERSION=$(KUBECTL_VERSION) yq eval -n '.machine.kubelet.image = "ghcr.io/siderolabs/kubelet:" + strenv(K8S_VERSION) + "-fat"' > $(ARTIFACTS)/kubelet-fat-patch.yaml

# Assets for releases

.PHONY: $(ARTIFACTS)/$(TALOS_RELEASE)
$(ARTIFACTS)/$(TALOS_RELEASE): $(ARTIFACTS)/$(TALOS_RELEASE)/vmlinuz $(ARTIFACTS)/$(TALOS_RELEASE)/initramfs.xz

# download release artifacts for specific version
$(ARTIFACTS)/$(TALOS_RELEASE)/%:
	@mkdir -p $(ARTIFACTS)/$(TALOS_RELEASE)/
	@case "$*" in \
		vmlinuz) \
			curl -L -o "$(ARTIFACTS)/$(TALOS_RELEASE)/$*" "https://github.com/siderolabs/talos/releases/download/$(TALOS_RELEASE)/vmlinuz-amd64" \
			;; \
		initramfs.xz) \
			curl -L -o "$(ARTIFACTS)/$(TALOS_RELEASE)/$*" "https://github.com/siderolabs/talos/releases/download/$(TALOS_RELEASE)/initramfs-amd64.xz" \
			;; \
	esac

.PHONY: release-artifacts
release-artifacts:
	@for release in $(RELEASES); do \
		$(MAKE) $(ARTIFACTS)/$$release TALOS_RELEASE=$$release; \
	done

# Utilities

.PHONY: rekres
rekres:
	@docker pull $(KRES_IMAGE)
	@docker run --rm --net=host --user $(shell id -u):$(shell id -g) -v $(PWD):/src -w /src -e GITHUB_TOKEN $(KRES_IMAGE)

.PHONY: conformance
conformance:
	@docker pull $(CONFORMANCE_IMAGE)
	@docker run --rm -it -v $(PWD):/src -w /src $(CONFORMANCE_IMAGE) enforce

.PHONY: release-notes
release-notes:
	ARTIFACTS=$(ARTIFACTS) ./hack/release.sh $@ $(ARTIFACTS)/RELEASE_NOTES.md $(TAG)

push: ## Pushes the installer, imager, talos and talosctl images to the configured container registry with the generated tag.
	@$(MAKE) installer PUSH=true
	@$(MAKE) imager PUSH=true
	@$(MAKE) talos PUSH=true
	@$(MAKE) talosctl-image PUSH=true

push-%: ## Pushes the installer, imager, talos and talosctl images to the configured container registry with the specified tag (e.g. push-latest).
	@$(MAKE) push IMAGE_TAG=$*

.PHONY: clean
clean: ## Cleans up all artifacts.
	@-rm -rf $(ARTIFACTS)

.PHONY: image-list
image-list: ## Prints a list of all images built by this Makefile with digests.
	@echo -n installer talos imager talosctl | xargs -d ' ' -I{} sh -c 'echo $(REGISTRY_AND_USERNAME)/{}:$(IMAGE_TAG)' | xargs -I{} sh -c 'echo {}@$$(crane digest {})'

.PHONY: sign-images
sign-images: ## Run cosign to sign all images built by this Makefile.
	@for image in $(shell $(MAKE) --quiet image-list REGISTRY_AND_USERNAME=$(REGISTRY_AND_USERNAME) IMAGE_TAG=$(IMAGE_TAG)); do \
		echo '==>' $$image; \
		cosign verify $$image --certificate-identity-regexp '@siderolabs\.com$$' --certificate-oidc-issuer https://accounts.google.com || \
			cosign sign --yes $$image; \
	done

.PHONY: reproducibility-test
reproducibility-test:
	@$(MAKE) reproducibility-test-local-initramfs
	@$(MAKE) reproducibility-test-docker-installer INSTALLER_ARCH=targetarch PLATFORM=linux/amd64
	@$(MAKE) reproducibility-test-docker-talos reproducibility-test-docker-imager reproducibility-test-docker-talosctl PLATFORM=linux/amd64

reproducibility-test-docker-%:
	@rm -rf _out1/ _out2/
	@mkdir -p _out1/ _out2/
	@$(MAKE) docker-$* DEST=_out1/
	@$(MAKE) docker-$* DEST=_out2/ TARGET_ARGS="--no-cache"
	@find _out1/ -type f | xargs -IFILE diffoscope FILE `echo FILE | sed 's/_out1/_out2/'`
	@rm -rf _out1/ _out2/

reproducibility-test-local-%:
	@rm -rf _out1/ _out2/
	@mkdir -p _out1/ _out2/
	@$(MAKE) local-$* DEST=_out1/
	@$(MAKE) local-$* DEST=_out2/ TARGET_ARGS="--no-cache"
	@find _out1/ -type f | xargs -IFILE diffoscope FILE `echo FILE | sed 's/_out1/_out2/'`
	@rm -rf _out1/ _out2/

.PHONY: ci-temp-release-tag
ci-temp-release-tag: ## Generates a temporary release tag for CI run.
	@if [ -n "$(CI_RELEASE_TAG)" -a -n "$${GITHUB_ENV}" ]; then \
		echo Setting temporary release tag "$(CI_RELEASE_TAG)"; \
		echo "TAG=$(CI_RELEASE_TAG)" >> "$${GITHUB_ENV}"; \
		echo "ABBREV_TAG=$(CI_RELEASE_TAG)" >> "$${GITHUB_ENV}"; \
	fi
