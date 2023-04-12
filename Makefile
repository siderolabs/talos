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
DOCKER_LOGIN_ENABLED ?= true
NAME = Talos

ARTIFACTS := _out
TOOLS ?= ghcr.io/siderolabs/tools:v1.5.0-alpha.0
PKGS ?= v1.5.0-alpha.0-4-g4eae958
EXTRAS ?= v1.5.0-alpha.0
# renovate: datasource=github-tags depName=golang/go
GO_VERSION ?= 1.20
# renovate: datasource=go depName=golang.org/x/tools
GOIMPORTS_VERSION ?= v0.7.0
# renovate: datasource=go depName=mvdan.cc/gofumpt
GOFUMPT_VERSION ?= v0.4.0
# renovate: datasource=go depName=github.com/golangci/golangci-lint
GOLANGCILINT_VERSION ?= v1.52.2
# renovate: datasource=go depName=golang.org/x/tools
STRINGER_VERSION ?= v0.7.0
# renovate: datasource=go depName=github.com/alvaroloes/enumer
ENUMER_VERSION ?= v1.1.2
# renovate: datasource=go depName=k8s.io/code-generator
DEEPCOPY_GEN_VERSION ?= v0.26.3
# renovate: datasource=go depName=github.com/planetscale/vtprotobuf
VTPROTOBUF_VERSION ?= v0.4.0
# renovate: datasource=go depName=github.com/siderolabs/deep-copy
DEEPCOPY_VERSION ?= v0.5.5
IMPORTVET ?= ghcr.io/siderolabs/importvet:2260533
# renovate: datasource=npm depName=markdownlint-cli
MARKDOWNLINTCLI_VERSION ?= 0.33.0
# renovate: datasource=npm depName=textlint
TEXTLINT_VERSION ?= 13.3.2
# renovate: datasource=npm depName=textlint-filter-rule-comments
TEXTLINT_FILTER_RULE_COMMENTS_VERSION ?= 1.2.2
# renovate: datasource=npm depName=textlint-rule-one-sentence-per-line
TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION ?= 2.0.0
OPERATING_SYSTEM := $(shell uname -s | tr "[:upper:]" "[:lower:]")
TALOSCTL_DEFAULT_TARGET := talosctl-$(OPERATING_SYSTEM)
INTEGRATION_TEST_DEFAULT_TARGET := integration-test-$(OPERATING_SYSTEM)
MODULE_SIG_VERIFY_DEFAULT_TARGET := module-sig-verify-$(OPERATING_SYSTEM)
INTEGRATION_TEST_PROVISION_DEFAULT_TARGET := integration-test-provision-$(OPERATING_SYSTEM)
# renovate: datasource=github-releases depName=kubernetes/kubernetes
KUBECTL_VERSION ?= v1.27.0
# renovate: datasource=github-releases depName=kastenhq/kubestr
KUBESTR_VERSION ?= v0.4.37
# renovate: datasource=github-releases depName=helm/helm
HELM_VERSION ?= v3.11.2
# renovate: datasource=github-releases depName=kubernetes-sigs/cluster-api
CLUSTERCTL_VERSION ?= 1.4.0
# renovate: datasource=github-releases depName=cilium/cilium-cli
CILIUM_CLI_VERSION ?= v0.13.2
KUBECTL_URL ?= https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/$(OPERATING_SYSTEM)/amd64/kubectl
KUBESTR_URL ?= https://github.com/kastenhq/kubestr/releases/download/$(KUBESTR_VERSION)/kubestr_$(subst v,,$(KUBESTR_VERSION))_Linux_amd64.tar.gz
HELM_URL ?= https://get.helm.sh/helm-$(HELM_VERSION)-linux-amd64.tar.gz
CLUSTERCTL_URL ?= https://github.com/kubernetes-sigs/cluster-api/releases/download/v$(CLUSTERCTL_VERSION)/clusterctl-$(OPERATING_SYSTEM)-amd64
CILIUM_CLI_URL ?= https://github.com/cilium/cilium-cli/releases/download/$(CILIUM_CLI_VERSION)/cilium-$(OPERATING_SYSTEM)-amd64.tar.gz
TESTPKGS ?= github.com/siderolabs/talos/...
RELEASES ?= v1.2.9 v1.3.6
SHORT_INTEGRATION_TEST ?=
CUSTOM_CNI_URL ?=
INSTALLER_ARCH ?= all
IMAGER_ARGS ?=
IMAGER_SYSTEM_EXTENSIONS ?=

CGO_ENABLED ?= 0
GO_BUILDFLAGS ?=
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
GO_BUILDFLAGS += -tags sidero.debug
else
GO_LDFLAGS += -s -w
endif

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
COMMON_ARGS += --build-arg=PKGS=$(PKGS)
COMMON_ARGS += --build-arg=EXTRAS=$(EXTRAS)
COMMON_ARGS += --build-arg=GOFUMPT_VERSION=$(GOFUMPT_VERSION)
COMMON_ARGS += --build-arg=GOIMPORTS_VERSION=$(GOIMPORTS_VERSION)
COMMON_ARGS += --build-arg=STRINGER_VERSION=$(STRINGER_VERSION)
COMMON_ARGS += --build-arg=ENUMER_VERSION=$(ENUMER_VERSION)
COMMON_ARGS += --build-arg=DEEPCOPY_GEN_VERSION=$(DEEPCOPY_GEN_VERSION)
COMMON_ARGS += --build-arg=VTPROTOBUF_VERSION=$(VTPROTOBUF_VERSION)
COMMON_ARGS += --build-arg=GOLANGCILINT_VERSION=$(GOLANGCILINT_VERSION)
COMMON_ARGS += --build-arg=DEEPCOPY_VERSION=$(DEEPCOPY_VERSION)
COMMON_ARGS += --build-arg=MARKDOWNLINTCLI_VERSION=$(MARKDOWNLINTCLI_VERSION)
COMMON_ARGS += --build-arg=TEXTLINT_VERSION=$(TEXTLINT_VERSION)
COMMON_ARGS += --build-arg=TEXTLINT_FILTER_RULE_COMMENTS_VERSION=$(TEXTLINT_FILTER_RULE_COMMENTS_VERSION)
COMMON_ARGS += --build-arg=TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION=$(TEXTLINT_RULE_ONE_SENTENCE_PER_LINE_VERSION)
COMMON_ARGS += --build-arg=TAG=$(TAG)
COMMON_ARGS += --build-arg=SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH)
COMMON_ARGS += --build-arg=ARTIFACTS=$(ARTIFACTS)
COMMON_ARGS += --build-arg=IMPORTVET=$(IMPORTVET)
COMMON_ARGS += --build-arg=TESTPKGS=$(TESTPKGS)
COMMON_ARGS += --build-arg=INSTALLER_ARCH=$(INSTALLER_ARCH)
COMMON_ARGS += --build-arg=CGO_ENABLED=$(CGO_ENABLED)
COMMON_ARGS += --build-arg=GO_BUILDFLAGS="$(GO_BUILDFLAGS)"
COMMON_ARGS += --build-arg=GO_LDFLAGS="$(GO_LDFLAGS)"
COMMON_ARGS += --build-arg=GOAMD64="$(GOAMD64)"
COMMON_ARGS += --build-arg=http_proxy=$(http_proxy)
COMMON_ARGS += --build-arg=https_proxy=$(https_proxy)
COMMON_ARGS += --build-arg=NAME=$(NAME)
COMMON_ARGS += --build-arg=SHA=$(SHA)
COMMON_ARGS += --build-arg=USERNAME=$(USERNAME)
COMMON_ARGS += --build-arg=REGISTRY=$(REGISTRY)
COMMON_ARGS += --build-arg=ABBREV_TAG=$(ABBREV_TAG)

CI_ARGS ?=

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

> Note: The security.insecure entitlement is only required, and used by the unit-tests target and targets which build container images
for applications using `img` tool.

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
	@$(MAKE) local-$@ DEST=./ PLATFORM=linux/amd64

.PHONY: docs
docs: ## Generates the documentation for machine config, and talosctl.
	@rm -rf docs/configuration/*
	@rm -rf docs/talosctl/*
	@$(MAKE) local-$@ DEST=./ PLATFORM=linux/amd64

.PHONY: docs-preview
docs-preview: ## Starts a local preview of the documentation using Hugo in docker
	@docker run --rm --interactive --tty \
	--volume $(PWD):/src --workdir /src/website \
	--publish 1313:1313 \
	klakegg/hugo:0.95.0-ext-alpine \
	server

# Local Artifacts

.PHONY: kernel
kernel: ## Outputs the kernel package contents (vmlinuz) to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false
	@-rm -rf $(ARTIFACTS)/modules

.PHONY: initramfs
initramfs: ## Builds the compressed initramfs and outputs it to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PUSH=false TARGET_ARGS="--allow security.insecure"

.PHONY: installer
installer: ## Builds the container image for the installer and outputs it to the registry.
	@INSTALLER_ARCH=targetarch  \
		$(MAKE) registry-$@ TARGET_ARGS="--allow security.insecure"

.PHONY: imager
imager: ## Builds the container image for the imager and outputs it to the registry.
	@$(MAKE) registry-$@ TARGET_ARGS="--allow security.insecure"

.PHONY: talos
talos: ## Builds the Talos container image and outputs it to the registry.
	@$(MAKE) registry-$@ TARGET_ARGS="--allow security.insecure"

.PHONY: talosctl-image
talosctl-image: ## Builds the talosctl container image and outputs it to the registry.
	@$(MAKE) registry-talosctl TARGET_ARGS="--allow security.insecure"

talosctl-%:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) PLATFORM=linux/amd64 PUSH=false NAME=Client

talosctl: $(TALOSCTL_DEFAULT_TARGET) ## Builds the talosctl binary for the local machine.

image-%: ## Builds the specified image. Valid options are aws, azure, digital-ocean, gcp, and vmware (e.g. image-aws)
	@docker pull $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG)
	@ . ./hack/imager.sh && \
	for platform in $(subst $(,),$(space),$(PLATFORM)); do \
		arch=$$(basename "$${platform}") && \
		tmpdir=$$(prepare_extension_images "$${platform}" $(IMAGER_SYSTEM_EXTENSIONS)) && \
		docker run --rm -v /dev:/dev -v "$${tmpdir}:/system/extensions" --privileged $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG) image --platform $* --arch $$arch --tar-to-stdout $(IMAGER_ARGS) | tar xz -C $(ARTIFACTS) ; \
		rm -rf "$${tmpdir}"; \
	done

images-essential: image-aws image-gcp image-metal ## Builds only essential images used in the CI (AWS, GCP, and Metal).

images: image-aws image-azure image-digital-ocean image-exoscale image-gcp image-hcloud image-metal image-nocloud image-openstack image-oracle image-scaleway image-upcloud image-vmware image-vultr ## Builds all known images (AWS, Azure, DigitalOcean, Exoscale, GCP, HCloud, Metal, NoCloud, Openstack, Oracle, Scaleway, UpCloud, Vultr and VMware).

sbc-%: ## Builds the specified SBC image. Valid options are rpi_4, rpi_generic, rock64, bananapi_m64, libretech_all_h3_cc_h5, rockpi_4, rockpi_4c, pine64, jetson_nano and nanopi_r4s (e.g. sbc-rpi_4)
	@docker pull $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG)
	@ . ./hack/imager.sh && \
	    tmpdir=$$(prepare_extension_images linux/arm64 $(IMAGER_SYSTEM_EXTENSIONS)) && \
		docker run --rm -v /dev:/dev -v "$${tmpdir}:/system/extensions" --privileged $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG) image --platform metal --arch arm64 --board $* --tar-to-stdout $(IMAGER_ARGS) | tar xz -C $(ARTIFACTS) ; \
		rm -rf "$${tmpdir}"

sbcs: sbc-rpi_4 sbc-rpi_generic sbc-rock64 sbc-bananapi_m64 sbc-libretech_all_h3_cc_h5 sbc-rockpi_4 sbc-rockpi_4c sbc-pine64 sbc-jetson_nano sbc-nanopi_r4s ## Builds all known SBC images (Raspberry Pi 4 Model B, Rock64, Banana Pi M64, Radxa ROCK Pi 4, Radxa ROCK Pi 4c, Pine64, Libre Computer Board ALL-H3-CC, Jetson Nano and Nano Pi R4S).

.PHONY: iso
iso: ## Builds the ISO and outputs it to the artifact directory.
	@docker pull $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG)
	@for platform in $(subst $(,),$(space),$(PLATFORM)); do \
		arch=`basename "$${platform}"` ; \
		docker run --rm -e SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) -i $(REGISTRY_AND_USERNAME)/imager:$(IMAGE_TAG) iso --arch $$arch --tar-to-stdout $(IMAGER_ARGS) | tar xz -C $(ARTIFACTS)  ; \
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
		-e TAG=$(TAG) -e ARTIFACTS=$(ARTIFACTS) \
		-e AWS_ACCESS_KEY_ID -e AWS_SECRET_ACCESS_KEY -e AWS_SVC_ACCT \
		-e AZURE_SVC_ACCT -e GCE_SVC_ACCT -e PACKET_AUTH_TOKEN \
		golang:$(GO_VERSION) \
		./hack/cloud-image-uploader.sh

# Code Quality

api-descriptors: ## Generates API descriptors used to detect breaking API changes.
	@$(MAKE) local-api-descriptors DEST=./ PLATFORM=linux/amd64

fmt-go: ## Formats the source code.
	@docker run --rm -it -v $(PWD):/src -w /src golang:$(GO_VERSION) bash -c "go install golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VERSION) && goimports -w -local github.com/siderolabs/talos . && go install mvdan.cc/gofumpt@$(GOFUMPT_VERSION) && gofumpt -w ."

fmt-protobuf: ## Formats protobuf files.
	@$(MAKE) local-fmt-protobuf DEST=./ PLATFORM=linux/amd64

fmt: ## Formats the source code and protobuf files.
	@$(MAKE) fmt-go fmt-protobuf

lint-%: ## Runs the specified linter. Valid options are go, protobuf, and markdown (e.g. lint-go).
	@$(MAKE) target-lint-$* PLATFORM=linux/amd64

lint: ## Runs linters on go, vulncheck, protobuf, and markdown file types.
	@$(MAKE) lint-go lint-vulncheck lint-protobuf lint-markdown

check-dirty: ## Verifies that source tree is not dirty
	@if test -n "`git status --porcelain`"; then echo "Source tree is dirty"; git status; exit 1 ; fi

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
	@$(MAKE) local-$(INTEGRATION_TEST_DEFAULT_TARGET) DEST=$(ARTIFACTS) PLATFORM=linux/amd64 WITH_RACE=true NAME=Client PUSH=false

$(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET) DEST=$(ARTIFACTS) PLATFORM=linux/amd64 WITH_RACE=true NAME=Client

$(ARTIFACTS)/$(MODULE_SIG_VERIFY_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(MODULE_SIG_VERIFY_DEFAULT_TARGET) DEST=$(ARTIFACTS) PLATFORM=linux/amd64

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

$(ARTIFACTS)/clusterctl:
	@mkdir -p $(ARTIFACTS)
	@curl -L -o $(ARTIFACTS)/clusterctl "$(CLUSTERCTL_URL)"
	@chmod +x $(ARTIFACTS)/clusterctl

$(ARTIFACTS)/cilium:
	@mkdir -p $(ARTIFACTS)
	@curl -L "$(CILIUM_CLI_URL)" | tar xzf - -C $(ARTIFACTS) cilium
	@chmod +x $(ARTIFACTS)/cilium

external-artifacts: $(ARTIFACTS)/kubectl $(ARTIFACTS)/clusterctl $(ARTIFACTS)/kubestr $(ARTIFACTS)/helm $(ARTIFACTS)/cilium $(ARTIFACTS)/$(MODULE_SIG_VERIFY_DEFAULT_TARGET)-amd64

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
		MODULE_SIG_VERIFY=$(PWD)/$(ARTIFACTS)/$(MODULE_SIG_VERIFY_DEFAULT_TARGET)-amd64 \
		KERNEL_MODULE_SIGNING_PUBLIC_KEY=$(PWD)/$(ARTIFACTS)/signing_key.x509 \
		SHORT_INTEGRATION_TEST=$(SHORT_INTEGRATION_TEST) \
		CUSTOM_CNI_URL=$(CUSTOM_CNI_URL) \
		KUBECTL=$(PWD)/$(ARTIFACTS)/kubectl \
		KUBESTR=$(PWD)/$(ARTIFACTS)/kubestr \
		HELM=$(PWD)/$(ARTIFACTS)/helm \
		CLUSTERCTL=$(PWD)/$(ARTIFACTS)/clusterctl \
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

.PHONY: conformance
conformance: ## Performs policy checks against the commit and source code.
	docker run --rm -it -v $(PWD):/src -w /src ghcr.io/siderolabs/conform:latest enforce

.PHONY: release-notes
release-notes:
	ARTIFACTS=$(ARTIFACTS) ./hack/release.sh $@ $(ARTIFACTS)/RELEASE_NOTES.md $(TAG)

.PHONY: login
login: ## Logs in to the configured container registry.
ifeq ($(DOCKER_LOGIN_ENABLED), true)
	@docker login --username "$(GHCR_USERNAME)" --password "$(GHCR_PASSWORD)" $(IMAGE_REGISTRY)
endif

push: login ## Pushes the installer, imager, talos and talosctl images to the configured container registry with the generated tag.
	@$(MAKE) installer PUSH=true
	@$(MAKE) imager PUSH=true
	@$(MAKE) talos PUSH=true
	@$(MAKE) talosctl-image PUSH=true

push-%: login ## Pushes the installer, imager, talos and talosctl images to the configured container registry with the specified tag (e.g. push-latest).
	@$(MAKE) push IMAGE_TAG=$*

.PHONY: clean
clean: ## Cleans up all artifacts.
	@-rm -rf $(ARTIFACTS)
