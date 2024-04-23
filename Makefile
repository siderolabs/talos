# THIS FILE WAS AUTOMATICALLY GENERATED, PLEASE DO NOT EDIT.
#
# Generated on 2024-04-23T18:08:19Z by kres ebc009d-dirty.

# common variables

SHA := $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG := $(shell git describe --tag --always --dirty --match v[0-9]\*)
ABBREV_TAG := $(shell git describe --tags >/dev/null 2>/dev/null && git describe --tag --always --match v[0-9]\* --abbrev=0 || echo 'undefined')
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
ARTIFACTS := _out
IMAGE_TAG ?= $(TAG)
OPERATING_SYSTEM := $(shell uname -s | tr '[:upper:]' '[:lower:]')
GOARCH := $(shell uname -m | sed 's/x86_64/amd64/' | sed 's/aarch64/arm64/')
SOURCE_DATE_EPOCH := $(shell git log -1 --pretty=%ct)
WITH_DEBUG ?= false
WITH_RACE ?= false
REGISTRY ?= ghcr.io
USERNAME ?= siderolabs
REGISTRY_AND_USERNAME ?= $(REGISTRY)/$(USERNAME)
PROTOBUF_GO_VERSION ?= 1.33.0
GRPC_GO_VERSION ?= 1.3.0
GRPC_GATEWAY_VERSION ?= 2.19.1
VTPROTOBUF_VERSION ?= 0.6.0
DEEPCOPY_VERSION ?= v0.5.6
GOLANGCILINT_VERSION ?= v1.57.2
GOFUMPT_VERSION ?= v0.6.0
GO_VERSION ?= 1.22.2
GOIMPORTS_VERSION ?= v0.20.0
GO_BUILDFLAGS ?=
GO_LDFLAGS ?=
CGO_ENABLED ?= 0
GOTOOLCHAIN ?= local
TESTPKGS ?= ./...
KRES_IMAGE ?= ghcr.io/siderolabs/kres:latest
CONFORMANCE_IMAGE ?= ghcr.io/siderolabs/conform:latest

# docker build settings

BUILD := docker buildx build
PLATFORM ?= linux/amd64
PROGRESS ?= auto
PUSH ?= false
CI_ARGS ?=
COMMON_ARGS = --file=Dockerfile
COMMON_ARGS += --provenance=false
COMMON_ARGS += --progress=$(PROGRESS)
COMMON_ARGS += --platform=$(PLATFORM)
COMMON_ARGS += --push=$(PUSH)
COMMON_ARGS += --build-arg=ARTIFACTS="$(ARTIFACTS)"
COMMON_ARGS += --build-arg=SHA="$(SHA)"
COMMON_ARGS += --build-arg=TAG="$(TAG)"
COMMON_ARGS += --build-arg=ABBREV_TAG="$(ABBREV_TAG)"
COMMON_ARGS += --build-arg=USERNAME="$(USERNAME)"
COMMON_ARGS += --build-arg=REGISTRY="$(REGISTRY)"
COMMON_ARGS += --build-arg=TOOLCHAIN="$(TOOLCHAIN)"
COMMON_ARGS += --build-arg=CGO_ENABLED="$(CGO_ENABLED)"
COMMON_ARGS += --build-arg=GO_BUILDFLAGS="$(GO_BUILDFLAGS)"
COMMON_ARGS += --build-arg=GO_LDFLAGS="$(GO_LDFLAGS)"
COMMON_ARGS += --build-arg=GOTOOLCHAIN="$(GOTOOLCHAIN)"
COMMON_ARGS += --build-arg=GOEXPERIMENT="$(GOEXPERIMENT)"
COMMON_ARGS += --build-arg=PROTOBUF_GO_VERSION="$(PROTOBUF_GO_VERSION)"
COMMON_ARGS += --build-arg=GRPC_GO_VERSION="$(GRPC_GO_VERSION)"
COMMON_ARGS += --build-arg=GRPC_GATEWAY_VERSION="$(GRPC_GATEWAY_VERSION)"
COMMON_ARGS += --build-arg=VTPROTOBUF_VERSION="$(VTPROTOBUF_VERSION)"
COMMON_ARGS += --build-arg=DEEPCOPY_VERSION="$(DEEPCOPY_VERSION)"
COMMON_ARGS += --build-arg=GOLANGCILINT_VERSION="$(GOLANGCILINT_VERSION)"
COMMON_ARGS += --build-arg=GOIMPORTS_VERSION="$(GOIMPORTS_VERSION)"
COMMON_ARGS += --build-arg=GOFUMPT_VERSION="$(GOFUMPT_VERSION)"
COMMON_ARGS += --build-arg=TESTPKGS="$(TESTPKGS)"
COMMON_ARGS += --build-arg=TOOLS="$(TOOLS)"
COMMON_ARGS += --build-arg=PKGS="$(PKGS)"
COMMON_ARGS += --build-arg=EXTRAS="$(EXTRAS)"
COMMON_ARGS += --build-arg=GOFUMPT_VERSION="$(GOFUMPT_VERSION)"
COMMON_ARGS += --build-arg=GOIMPORTS_VERSION="$(GOIMPORTS_VERSION)"
COMMON_ARGS += --build-arg=STRINGER_VERSION="$(STRINGER_VERSION)"
COMMON_ARGS += --build-arg=ENUMER_VERSION="$(ENUMER_VERSION)"
COMMON_ARGS += --build-arg=DEEPCOPY_GEN_VERSION="$(DEEPCOPY_GEN_VERSION)"
COMMON_ARGS += --build-arg=VTPROTOBUF_VERSION="$(VTPROTOBUF_VERSION)"
COMMON_ARGS += --build-arg=IMPORTVET_VERSION="$(IMPORTVET_VERSION)"
COMMON_ARGS += --build-arg=GOLANGCILINT_VERSION="$(GOLANGCILINT_VERSION)"
COMMON_ARGS += --build-arg=DEEPCOPY_VERSION="$(DEEPCOPY_VERSION)"
COMMON_ARGS += --build-arg=MARKDOWNLINTCLI_VERSION="$(MARKDOWNLINTCLI_VERSION)"
COMMON_ARGS += --build-arg=TEXTLINT_VERSION="$(TEXTLINT_VERSION)"
COMMON_ARGS += --build-arg=TEXTLINT_FILTER_RULE_COMMENTS_VERSION="$(TEXTLINT_FILTER_RULE_COMMENTS_VERSION)"
COMMON_ARGS += --build-arg=TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION="$(TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION)"
COMMON_ARGS += --build-arg=TAG="$(TAG)"
COMMON_ARGS += --build-arg=SOURCE_DATE_EPOCH="$(SOURCE_DATE_EPOCH)"
COMMON_ARGS += --build-arg=ARTIFACTS="$(ARTIFACTS)"
COMMON_ARGS += --build-arg=TESTPKGS="$(TESTPKGS)"
COMMON_ARGS += --build-arg=INSTALLER_ARCH="$(INSTALLER_ARCH)"
COMMON_ARGS += --build-arg=GOAMD64="$(GOAMD64)"
COMMON_ARGS += --build-arg=http_proxy="$(http_proxy)"
COMMON_ARGS += --build-arg=https_proxy="$(https_proxy)"
COMMON_ARGS += --build-arg=NAME="$(NAME)"
COMMON_ARGS += --build-arg=SHA="$(SHA)"
COMMON_ARGS += --build-arg=USERNAME="$(USERNAME)"
COMMON_ARGS += --build-arg=REGISTRY="$(REGISTRY)"
COMMON_ARGS += --build-arg=PKGS_PREFIX="$(PKGS_PREFIX)"
COMMON_ARGS += --build-arg=PKG_FHS="$(PKG_FHS)"
COMMON_ARGS += --build-arg=PKG_CA_CERTIFICATES="$(PKG_CA_CERTIFICATES)"
COMMON_ARGS += --build-arg=PKG_CRYPTSETUP="$(PKG_CRYPTSETUP)"
COMMON_ARGS += --build-arg=PKG_CONTAINERD="$(PKG_CONTAINERD)"
COMMON_ARGS += --build-arg=PKG_DOSFSTOOLS="$(PKG_DOSFSTOOLS)"
COMMON_ARGS += --build-arg=PKG_EUDEV="$(PKG_EUDEV)"
COMMON_ARGS += --build-arg=PKG_GRUB="$(PKG_GRUB)"
COMMON_ARGS += --build-arg=PKG_SD_BOOT="$(PKG_SD_BOOT)"
COMMON_ARGS += --build-arg=PKG_IPTABLES="$(PKG_IPTABLES)"
COMMON_ARGS += --build-arg=PKG_IPXE="$(PKG_IPXE)"
COMMON_ARGS += --build-arg=PKG_LIBINIH="$(PKG_LIBINIH)"
COMMON_ARGS += --build-arg=PKG_LIBJSON_C="$(PKG_LIBJSON_C)"
COMMON_ARGS += --build-arg=PKG_LIBPOPT="$(PKG_LIBPOPT)"
COMMON_ARGS += --build-arg=PKG_LIBURCU="$(PKG_LIBURCU)"
COMMON_ARGS += --build-arg=PKG_OPENSSL="$(PKG_OPENSSL)"
COMMON_ARGS += --build-arg=PKG_LIBSECCOMP="$(PKG_LIBSECCOMP)"
COMMON_ARGS += --build-arg=PKG_LINUX_FIRMWARE="$(PKG_LINUX_FIRMWARE)"
COMMON_ARGS += --build-arg=PKG_LVM2="$(PKG_LVM2)"
COMMON_ARGS += --build-arg=PKG_LIBAIO="$(PKG_LIBAIO)"
COMMON_ARGS += --build-arg=PKG_MUSL="$(PKG_MUSL)"
COMMON_ARGS += --build-arg=PKG_RUNC="$(PKG_RUNC)"
COMMON_ARGS += --build-arg=PKG_XFSPROGS="$(PKG_XFSPROGS)"
COMMON_ARGS += --build-arg=PKG_UTIL_LINUX="$(PKG_UTIL_LINUX)"
COMMON_ARGS += --build-arg=PKG_KMOD="$(PKG_KMOD)"
COMMON_ARGS += --build-arg=PKG_U_BOOT="$(PKG_U_BOOT)"
COMMON_ARGS += --build-arg=PKG_RASPBERYPI_FIRMWARE="$(PKG_RASPBERYPI_FIRMWARE)"
COMMON_ARGS += --build-arg=PKG_KERNEL="$(PKG_KERNEL)"
COMMON_ARGS += --build-arg=PKG_TALOSCTL_CNI_BUNDLE_INSTALL="$(PKG_TALOSCTL_CNI_BUNDLE_INSTALL)"
COMMON_ARGS += --build-arg=ABBREV_TAG="$(ABBREV_TAG)"
TOOLCHAIN ?= docker.io/golang:1.22-alpine

# extra variables

NAME ?= Talos
GOAMD64 ?= v2
CLOUD_IMAGES_EXTRA_ARGS ?=
TOOLS ?= ghcr.io/siderolabs/tools:v1.8.0-alpha.0
PKGS_PREFIX ?= ghcr.io/siderolabs
PKGS ?= v1.8.0-alpha.0-3-g010913b
EXTRAS ?= v1.8.0-alpha.0
TALOSCTL_DEFAULT_TARGET ?= talosctl-$(OPERATING_SYSTEM)-$(GOARCH)
TALOSCTL_EXECUTABLE ?= $(PWD)/$(ARTIFACTS)/$(TALOSCTL_DEFAULT_TARGET)
INTEGRATION_TEST_DEFAULT_TARGET ?= integration-test-$(OPERATING_SYSTEM)-$(GOARCH)
INTEGRATION_TEST_PROVISION_DEFAULT_TARGET ?= integration-test-provision-$(OPERATING_SYSTEM)-$(GOARCH)
PKG_FHS ?= $(PKGS_PREFIX)/fhs:$(PKGS)
PKG_CA_CERTIFICATES ?= $(PKGS_PREFIX)/ca-certificates:$(PKGS)
PKG_CRYPTSETUP ?= $(PKGS_PREFIX)/cryptsetup:$(PKGS)
PKG_CONTAINERD ?= $(PKGS_PREFIX)/containerd:$(PKGS)
PKG_DOSFSTOOLS ?= $(PKGS_PREFIX)/dosfstools:$(PKGS)
PKG_EUDEV ?= $(PKGS_PREFIX)/eudev:$(PKGS)
PKG_GRUB ?= $(PKGS_PREFIX)/grub:$(PKGS)
PKG_SD_BOOT ?= $(PKGS_PREFIX)/sd-boot:$(PKGS)
PKG_IPTABLES ?= $(PKGS_PREFIX)/iptables:$(PKGS)
PKG_IPXE ?= $(PKGS_PREFIX)/ipxe:$(PKGS)
PKG_LIBINIH ?= $(PKGS_PREFIX)/libinih:$(PKGS)
PKG_LIBJSON_C ?= $(PKGS_PREFIX)/libjson-c:$(PKGS)
PKG_LIBPOPT ?= $(PKGS_PREFIX)/libpopt:$(PKGS)
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
PKG_KERNEL ?= $(PKGS_PREFIX)/kernel:$(PKGS)
PKG_TALOSCTL_CNI_BUNDLE_INSTALL ?= $(PKGS_PREFIX)/talosctl-cni-bundle-install:$(EXTRAS)
GO_VERSION ?= 1.22
GOIMPORTS_VERSION ?= v0.19.0
GOFUMPT_VERSION ?= v0.6.0
GOLANGCILINT_VERSION ?= v1.57.2
STRINGER_VERSION ?= v0.19.0
ENUMER_VERSION ?= v1.5.9
DEEPCOPY_GEN_VERSION ?= v0.29.3
VTPROTOBUF_VERSION ?= v0.6.0
DEEPCOPY_VERSION ?= v0.5.6
IMPORTVET_VERSION ?= v0.2.0
MARKDOWNLINTCLI_VERSION ?= 0.39.0
TEXTLINT_VERSION ?= 14.0.4
TEXTLINT_FILTER_RULE_COMMENTS_VERSION ?= 1.2.2
TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION ?= 2.0.0
HUGO_VERSION ?= 0.111.3-ext-alpine
KUBECTL_VERSION ?= v1.30.0
KUBESTR_VERSION ?= v0.4.44
HELM_VERSION ?= v3.14.3
CLUSTERCTL_VERSION ?= 1.6.3
CILIUM_CLI_VERSION ?= v0.16.4
KUBECTL_URL ?= https://dl.k8s.io/release/$(KUBECTL_VERSION)/bin/$(OPERATING_SYSTEM)/amd64/kubectl
KUBESTR_URL ?= https://github.com/kastenhq/kubestr/releases/download/$(KUBESTR_VERSION)/kubestr_$(subst v,,$(KUBESTR_VERSION))_Linux_amd64.tar.gz
HELM_URL ?= https://get.helm.sh/helm-$(HELM_VERSION)-linux-amd64.tar.gz
CLUSTERCTL_URL ?= https://github.com/kubernetes-sigs/cluster-api/releases/download/v$(CLUSTERCTL_VERSION)/clusterctl-$(OPERATING_SYSTEM)-amd64
CILIUM_CLI_URL ?= https://github.com/cilium/cilium-cli/releases/download/$(CILIUM_CLI_VERSION)/cilium-$(OPERATING_SYSTEM)-amd64.tar.gz
TESTPKGS ?= github.com/siderolabs/talos/...
RELEASES ?= v1.6.7 v1.7.0
SHORT_INTEGRATION_TEST ?=
CUSTOM_CNI_URL ?=
INSTALLER_ARCH ?= all
IMAGER_ARGS ?=

# extra buildtags

GO_BUILDFLAGS += -tags tcell_minimal,grpcnotrace

# help menu

export define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- docker (19.03 or higher)

## Creating a Builder Instance

The build process makes use of experimental Docker features (buildx).
To enable experimental features, add 'experimental: "true"' to '/etc/docker/daemon.json' on
Linux or enable experimental features in Docker GUI for Windows or Mac.

To create a builder instance, run:

	docker buildx create --name local --use

If running builds that needs to be cached aggresively create a builder instance with the following:

	docker buildx create --name local --use --config=config.toml

config.toml contents:

[worker.oci]
  gc = true
  gckeepstorage = 50000

  [[worker.oci.gcpolicy]]
    keepBytes = 10737418240
    keepDuration = 604800
    filters = [ "type==source.local", "type==exec.cachemount", "type==source.git.checkout"]
  [[worker.oci.gcpolicy]]
    all = true
    keepBytes = 53687091200

If you already have a compatible builder instance, you may use that instead.

## Artifacts

All artifacts will be output to ./$(ARTIFACTS). Images will be tagged with the
registry "$(REGISTRY)", username "$(USERNAME)", and a dynamic tag (e.g. $(IMAGE):$(IMAGE_TAG)).
The registry and username can be overridden by exporting REGISTRY, and USERNAME
respectively.

endef

ifneq (, $(filter $(WITH_RACE), t true TRUE y yes 1))
GO_BUILDFLAGS += -race
CGO_ENABLED := 1
GO_LDFLAGS += -linkmode=external -extldflags '-static'
endif

ifneq (, $(filter $(WITH_DEBUG), t true TRUE y yes 1))
GO_BUILDFLAGS += -tags sidero.debug
else
GO_LDFLAGS += -s
endif

all: unit-tests unit-tests-hack-cloud-image-uploader unit-tests-hack-docgen unit-tests-hack-gotagsrewrite unit-tests-hack-module-sig-verify unit-tests-hack-structprotogen unit-tests-pkg-machinery installer image-installer talosctl image-talosctl external-artifacts $(ARTIFACTS)/cilium $(ARTIFACTS)/clusterctl $(ARTIFACTS)/helm $(ARTIFACTS)/kubectl $(ARTIFACTS)/kubestr generate docs uki-certs talosctl-all hack-test docs-preview initramfs sd-boot sd-stub installer imager $(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64 $(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64 lint-protobuf talosctl-cni-bundle image-% iso secureboot-iso images-essential images e2e-% lint

$(ARTIFACTS):  ## Creates artifacts directory.
	@mkdir -p $(ARTIFACTS)

.PHONY: clean
clean:  ## Cleans up all artifacts.
	@rm -rf $(ARTIFACTS)

check-dirty:  ## Verifies that source tree is not dirty
	@if test -n "`git status --porcelain`"; then echo "Source tree is dirty"; git status; git diff; exit 1 ; fi

target-%:  ## Builds the specified target defined in the Dockerfile. The build result will only remain in the build cache.
	@$(BUILD) --target=$* $(COMMON_ARGS) $(TARGET_ARGS) $(CI_ARGS) .

local-%:  ## Builds the specified target defined in the Dockerfile using the local output type. The build result will be output to the specified local destination.
	@$(MAKE) target-$* TARGET_ARGS="--output=type=local,dest=$(DEST) $(TARGET_ARGS)"

lint-golangci-lint:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-gofumpt:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

.PHONY: fmt
fmt:  ## Formats the source code
	@docker run --rm -it -v $(PWD):/src -w /src golang:$(GO_VERSION) \
		bash -c "export GOTOOLCHAIN=local; \
		export GO111MODULE=on; export GOPROXY=https://proxy.golang.org; \
		go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION) && \
		gofumpt -w ."

lint-govulncheck:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-goimports:  ## Runs goimports linter.
	@$(MAKE) target-$@

lint-golangci-lint-hack-cloud-image-uploader:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-gofumpt-hack-cloud-image-uploader:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

lint-govulncheck-hack-cloud-image-uploader:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-goimports-hack-cloud-image-uploader:  ## Runs goimports linter.
	@$(MAKE) target-$@

lint-golangci-lint-hack-docgen:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-gofumpt-hack-docgen:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

lint-govulncheck-hack-docgen:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-goimports-hack-docgen:  ## Runs goimports linter.
	@$(MAKE) target-$@

lint-golangci-lint-hack-gotagsrewrite:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-gofumpt-hack-gotagsrewrite:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

lint-govulncheck-hack-gotagsrewrite:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-goimports-hack-gotagsrewrite:  ## Runs goimports linter.
	@$(MAKE) target-$@

lint-golangci-lint-hack-module-sig-verify:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-gofumpt-hack-module-sig-verify:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

lint-govulncheck-hack-module-sig-verify:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-goimports-hack-module-sig-verify:  ## Runs goimports linter.
	@$(MAKE) target-$@

lint-golangci-lint-hack-structprotogen:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-gofumpt-hack-structprotogen:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

lint-govulncheck-hack-structprotogen:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-goimports-hack-structprotogen:  ## Runs goimports linter.
	@$(MAKE) target-$@

lint-golangci-lint-pkg-machinery:  ## Runs golangci-lint linter.
	@$(MAKE) target-$@

lint-gofumpt-pkg-machinery:  ## Runs gofumpt linter.
	@$(MAKE) target-$@

lint-govulncheck-pkg-machinery:  ## Runs govulncheck linter.
	@$(MAKE) target-$@

lint-goimports-pkg-machinery:  ## Runs goimports linter.
	@$(MAKE) target-$@

.PHONY: base
base:  ## Prepare base toolchain
	@$(MAKE) target-$@

.PHONY: unit-tests
unit-tests:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-race
unit-tests-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-cloud-image-uploader
unit-tests-hack-cloud-image-uploader:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-cloud-image-uploader-race
unit-tests-hack-cloud-image-uploader-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-docgen
unit-tests-hack-docgen:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-docgen-race
unit-tests-hack-docgen-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-gotagsrewrite
unit-tests-hack-gotagsrewrite:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-gotagsrewrite-race
unit-tests-hack-gotagsrewrite-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-module-sig-verify
unit-tests-hack-module-sig-verify:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-module-sig-verify-race
unit-tests-hack-module-sig-verify-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-structprotogen
unit-tests-hack-structprotogen:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-hack-structprotogen-race
unit-tests-hack-structprotogen-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-pkg-machinery
unit-tests-pkg-machinery:  ## Performs unit tests
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)  TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-pkg-machinery-race
unit-tests-pkg-machinery-race:  ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@  TARGET_ARGS="--allow security.insecure"

.PHONY: coverage
coverage:  ## Upload coverage data to codecov.io.
	bash -c "bash <(curl -s https://codecov.io/bash) -f $(ARTIFACTS)/coverage-unit-tests.txt -X fix"
	bash -c "bash <(curl -s https://codecov.io/bash) -f $(ARTIFACTS)/coverage-unit-tests-hack-cloud-image-uploader.txt -X fix"
	bash -c "bash <(curl -s https://codecov.io/bash) -f $(ARTIFACTS)/coverage-unit-tests-hack-docgen.txt -X fix"
	bash -c "bash <(curl -s https://codecov.io/bash) -f $(ARTIFACTS)/coverage-unit-tests-hack-gotagsrewrite.txt -X fix"
	bash -c "bash <(curl -s https://codecov.io/bash) -f $(ARTIFACTS)/coverage-unit-tests-hack-module-sig-verify.txt -X fix"
	bash -c "bash <(curl -s https://codecov.io/bash) -f $(ARTIFACTS)/coverage-unit-tests-hack-structprotogen.txt -X fix"
	bash -c "bash <(curl -s https://codecov.io/bash) -f $(ARTIFACTS)/coverage-unit-tests-pkg-machinery.txt -X fix"

.PHONY: $(ARTIFACTS)/installer-linux-amd64
$(ARTIFACTS)/installer-linux-amd64:
	@$(MAKE) local-installer-linux-amd64 DEST=$(ARTIFACTS)

.PHONY: installer-linux-amd64
installer-linux-amd64: $(ARTIFACTS)/installer-linux-amd64  ## Builds executable for installer-linux-amd64.

.PHONY: installer
installer: installer-linux-amd64  ## Builds executables for installer.

.PHONY: lint-markdown
lint-markdown:  ## Runs markdownlint.
	@$(MAKE) target-$@

.PHONY: lint
lint: lint-golangci-lint lint-gofumpt lint-govulncheck lint-goimports lint-golangci-lint-hack-cloud-image-uploader lint-gofumpt-hack-cloud-image-uploader lint-govulncheck-hack-cloud-image-uploader lint-goimports-hack-cloud-image-uploader lint-golangci-lint-hack-docgen lint-gofumpt-hack-docgen lint-govulncheck-hack-docgen lint-goimports-hack-docgen lint-golangci-lint-hack-gotagsrewrite lint-gofumpt-hack-gotagsrewrite lint-govulncheck-hack-gotagsrewrite lint-goimports-hack-gotagsrewrite lint-golangci-lint-hack-module-sig-verify lint-gofumpt-hack-module-sig-verify lint-govulncheck-hack-module-sig-verify lint-goimports-hack-module-sig-verify lint-golangci-lint-hack-structprotogen lint-gofumpt-hack-structprotogen lint-govulncheck-hack-structprotogen lint-goimports-hack-structprotogen lint-golangci-lint-pkg-machinery lint-gofumpt-pkg-machinery lint-govulncheck-pkg-machinery lint-goimports-pkg-machinery lint-markdown lint-protobuf  ## Run all linters for the project.

.PHONY: image-installer
image-installer:  ## Builds image for installer.
	@$(MAKE) target-$@ TARGET_ARGS="--tag=$(REGISTRY)/$(USERNAME)/installer:$(IMAGE_TAG)"

.PHONY: $(ARTIFACTS)/talosctl-darwin-amd64
$(ARTIFACTS)/talosctl-darwin-amd64:
	@$(MAKE) local-talosctl-darwin-amd64 DEST=$(ARTIFACTS) NAME=Client

.PHONY: talosctl-darwin-amd64
talosctl-darwin-amd64: $(ARTIFACTS)/talosctl-darwin-amd64  ## Builds executable for talosctl-darwin-amd64.

.PHONY: $(ARTIFACTS)/talosctl-darwin-arm64
$(ARTIFACTS)/talosctl-darwin-arm64:
	@$(MAKE) local-talosctl-darwin-arm64 DEST=$(ARTIFACTS) NAME=Client

.PHONY: talosctl-darwin-arm64
talosctl-darwin-arm64: $(ARTIFACTS)/talosctl-darwin-arm64  ## Builds executable for talosctl-darwin-arm64.

.PHONY: $(ARTIFACTS)/talosctl-freebsd-amd64
$(ARTIFACTS)/talosctl-freebsd-amd64:
	@$(MAKE) local-talosctl-freebsd-amd64 DEST=$(ARTIFACTS) NAME=Client

.PHONY: talosctl-freebsd-amd64
talosctl-freebsd-amd64: $(ARTIFACTS)/talosctl-freebsd-amd64  ## Builds executable for talosctl-freebsd-amd64.

.PHONY: $(ARTIFACTS)/talosctl-freebsd-arm64
$(ARTIFACTS)/talosctl-freebsd-arm64:
	@$(MAKE) local-talosctl-freebsd-arm64 DEST=$(ARTIFACTS) NAME=Client

.PHONY: talosctl-freebsd-arm64
talosctl-freebsd-arm64: $(ARTIFACTS)/talosctl-freebsd-arm64  ## Builds executable for talosctl-freebsd-arm64.

.PHONY: $(ARTIFACTS)/talosctl-linux-amd64
$(ARTIFACTS)/talosctl-linux-amd64:
	@$(MAKE) local-talosctl-linux-amd64 DEST=$(ARTIFACTS) NAME=Client

.PHONY: talosctl-linux-amd64
talosctl-linux-amd64: $(ARTIFACTS)/talosctl-linux-amd64  ## Builds executable for talosctl-linux-amd64.

.PHONY: $(ARTIFACTS)/talosctl-linux-arm64
$(ARTIFACTS)/talosctl-linux-arm64:
	@$(MAKE) local-talosctl-linux-arm64 DEST=$(ARTIFACTS) NAME=Client

.PHONY: talosctl-linux-arm64
talosctl-linux-arm64: $(ARTIFACTS)/talosctl-linux-arm64  ## Builds executable for talosctl-linux-arm64.

.PHONY: $(ARTIFACTS)/talosctl-windows-amd64.exe
$(ARTIFACTS)/talosctl-windows-amd64.exe:
	@$(MAKE) local-talosctl-windows-amd64.exe DEST=$(ARTIFACTS) NAME=Client

.PHONY: talosctl-windows-amd64.exe
talosctl-windows-amd64.exe: $(ARTIFACTS)/talosctl-windows-amd64.exe  ## Builds executable for talosctl-windows-amd64.exe.

.PHONY: talosctl
talosctl: talosctl-darwin-amd64 talosctl-darwin-arm64 talosctl-freebsd-amd64 talosctl-freebsd-arm64 talosctl-linux-amd64 talosctl-linux-arm64 talosctl-windows-amd64.exe  ## Builds executables for talosctl.

.PHONY: image-talosctl
image-talosctl:  ## Builds image for talosctl.
	@$(MAKE) target-$@ TARGET_ARGS="--tag=$(REGISTRY)/$(USERNAME)/talosctl:$(IMAGE_TAG)"

external-artifacts: $(ARTIFACTS)/cilium $(ARTIFACTS)/clusterctl $(ARTIFACTS)/helm $(ARTIFACTS)/kubectl $(ARTIFACTS)/kubestr

$(ARTIFACTS)/cilium: $(ARTIFACTS)
	@curl -L "$(CILIUM_CLI_URL)" | tar xzf - -C $(ARTIFACTS) cilium
	@chmod +x $(ARTIFACTS)/cilium

$(ARTIFACTS)/clusterctl: $(ARTIFACTS)
	@curl -L -o $(ARTIFACTS)/clusterctl "$(CLUSTERCTL_URL)"
	@chmod +x $(ARTIFACTS)/clusterctl

$(ARTIFACTS)/helm: $(ARTIFACTS)
	@curl -L "$(HELM_URL)" | tar xzf - -C $(ARTIFACTS) --strip-components=1 linux-amd64/helm
	@chmod +x $(ARTIFACTS)/helm

$(ARTIFACTS)/kubectl: $(ARTIFACTS)
	@curl -L -o $(ARTIFACTS)/kubectl "$(KUBECTL_URL)"
	@chmod +x $(ARTIFACTS)/kubectl

$(ARTIFACTS)/kubestr: $(ARTIFACTS)
	@curl -L "$(KUBESTR_URL)" | tar xzf - -C $(ARTIFACTS) kubestr
	@chmod +x $(ARTIFACTS)/kubestr

.PHONY: generate
generate:
	@$(MAKE) local-$@ DEST=./ PLATFORM=linux/amd64

.PHONY: docs
docs:
	@rm -rf docs/configuration/*
	@rm -rf docs/talosctl/*
	@$(MAKE) local-$@ DEST=./ PLATFORM=linux/amd64

.PHONY: uki-certs
uki-certs: $(TALOSCTL_DEFAULT_TARGET)
	@$(TALOSCTL_EXECUTABLE) gen secureboot uki
	@$(TALOSCTL_EXECUTABLE) gen secureboot pcr
	@$(TALOSCTL_EXECUTABLE) gen secureboot database

.PHONY: talosctl-all
talosctl-all:
	@$(MAKE) local-talosctl-all DEST=$(ARTIFACTS) PUSH=false NAME=Client

hack-test:
	@./hack/test/$*.sh

.PHONY: docs-preview
docs-preview:
	@docker run --rm --interactive --tty --user $(shell id -u):$(shell id -g) --volume $(PWD):/src --workdir /src/website --publish 1313:1313 klakegg/hugo:$(HUGO_VERSION) server

.PHONY: initramfs
initramfs:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false

.PHONY: sd-boot
sd-boot:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false

.PHONY: sd-stub
sd-stub:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false

.PHONY: installer
installer:
	@INSTALLER_ARCH=targetarch $(MAKE) image-installer

.PHONY: imager
imager:
	@INSTALLER_ARCH=targetarch $(MAKE) image-imager

.PHONY: $(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64
$(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(INTEGRATION_TEST_DEFAULT_TARGET) DEST=$(ARTIFACTS) PLATFORM=linux/amd64 WITH_RACE=true NAME=Client PUSH=false

.PHONY: $(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64
$(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET) DEST=$(ARTIFACTS) PLATFORM=linux/amd64 WITH_RACE=true NAME=Client

.PHONY: lint-protobuf
lint-protobuf:
	@$(MAKE) target-lint-protobuf PLATFORM=linux/amd64

talosctl-cni-bundle:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)
	
	@for platform in $(shell echo $(PLATFORM) | tr "," " "); do \
	  arch=`basename $$platform` ; \
	
	  tar  -C $(ARTIFACTS)/talosctl-cni-bundle-$${arch} -czf $(ARTIFACTS)/talosctl-cni-bundle-$${arch}.tar.gz . ; \
	done

.PHONY: image-%
image-%:
	@docker pull $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG)
	
	@for platform in $(shell echo $(PLATFORM) | tr "," " "); do \
	  arch=`basename $$platform` ; \
	
	  docker run --rm -t -v /dev:/dev -v $(PWD)/$(ARTIFACTS):/secureboot:ro -v $(PWD)/$(ARTIFACTS):/out --network=host --privileged $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG) $* --arch $$arch $(IMAGER_ARGS) ; \
	done

.PHONY: iso
iso: image-iso

.PHONY: secureboot-iso
secureboot-iso: image-secureboot-iso

.PHONY: images-essential
images-essential: image-aws image-azure image-gcp image-metal secureboot-installer

.PHONY: images
images: image-akamai image-aws image-azure image-digital-ocean image-exoscale image-gcp image-hcloud image-iso image-metal image-nocloud image-opennebula image-openstack image-oracle image-scaleway image-upcloud image-vmware image-vultr

.PHONY: e2e-%
e2e-%: $(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64 external-artifacts
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
	  MODULE_SIG_VERIFY=$(PWD)/$(ARTIFACTS)/$(MODULE_SIG_VERIFY_DEFAULT_TARGET)-amd64 \
	  KERNEL_MODULE_SIGNING_PUBLIC_KEY=$(PWD)/$(ARTIFACTS)/signing_key.x509 \
	  SHORT_INTEGRATION_TEST=$(SHORT_INTEGRATION_TEST) \
	  CUSTOM_CNI_URL=$(CUSTOM_CNI_URL) \
	  KUBECTL=$(PWD)/$(ARTIFACTS)/kubectl \
	  KUBESTR=$(PWD)/$(ARTIFACTS)/kubestr \
	  HELM=$(PWD)/$(ARTIFACTS)/helm \
	  CLUSTERCTL=$(PWD)/$(ARTIFACTS)/clusterctl \
	  CILIUM_CLI=$(PWD)/$(ARTIFACTS)/cilium

.PHONY: rekres
rekres:
	@docker pull $(KRES_IMAGE)
	@docker run --rm --net=host --user $(shell id -u):$(shell id -g) -v $(PWD):/src -w /src -e GITHUB_TOKEN $(KRES_IMAGE)

.PHONY: help
help:  ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: release-notes
release-notes: $(ARTIFACTS)
	@ARTIFACTS=$(ARTIFACTS) ./hack/release.sh $@ $(ARTIFACTS)/RELEASE_NOTES.md $(TAG)

.PHONY: conformance
conformance:
	@docker pull $(CONFORMANCE_IMAGE)
	@docker run --rm -it -v $(PWD):/src -w /src $(CONFORMANCE_IMAGE) enforce

