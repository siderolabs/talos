REGISTRY ?= docker.io
USERNAME ?= autonomy
SHA ?= $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG ?= $(shell git describe --tag --always --dirty)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
REGISTRY_AND_USERNAME := $(REGISTRY)/$(USERNAME)
DOCKER_LOGIN_ENABLED ?= true

ARTIFACTS := _out
TOOLS ?= autonomy/tools:v0.1.0-2-g4d7ce7d
GO_VERSION ?= 1.14
OPERATING_SYSTEM := $(shell uname -s | tr "[:upper:]" "[:lower:]")
OSCTL_DEFAULT_TARGET := osctl-$(OPERATING_SYSTEM)
INTEGRATION_TEST_DEFAULT_TARGET := integration-test-$(OPERATING_SYSTEM)
INTEGRATION_TEST_PROVISION_DEFAULT_TARGET := integration-test-provision-$(OPERATING_SYSTEM)
KUBECTL_URL ?= https://storage.googleapis.com/kubernetes-release/release/v1.17.1/bin/$(OPERATING_SYSTEM)/amd64/kubectl
SONOBUOY_VERSION ?= 0.17.1
SONOBUOY_URL ?= https://github.com/heptio/sonobuoy/releases/download/v$(SONOBUOY_VERSION)/sonobuoy_$(SONOBUOY_VERSION)_$(OPERATING_SYSTEM)_amd64.tar.gz
TESTPKGS ?= ./...
RELEASES ?= v0.3.2 v0.4.0-alpha.5

BUILD := docker buildx build
PLATFORM ?= linux/amd64
PROGRESS ?= auto
PUSH ?= false
COMMON_ARGS := --file=Dockerfile
COMMON_ARGS += --progress=$(PROGRESS)
COMMON_ARGS += --platform=$(PLATFORM)
COMMON_ARGS += --push=$(PUSH)
COMMON_ARGS += --build-arg=TOOLS=$(TOOLS)
COMMON_ARGS += --build-arg=SHA=$(SHA)
COMMON_ARGS += --build-arg=TAG=$(TAG)
COMMON_ARGS += --build-arg=GO_VERSION=$(GO_VERSION)
COMMON_ARGS += --build-arg=ARTIFACTS=$(ARTIFACTS)
COMMON_ARGS += --build-arg=TESTPKGS=$(TESTPKGS)
COMMON_ARGS += --build-arg=USERNAME=$(USERNAME)
COMMON_ARGS += --build-arg=http_proxy=$(http_proxy)
COMMON_ARGS += --build-arg=https_proxy=$(https_proxy)

all: initramfs kernel installer osctl talos

# Help Menu

define HELP_MENU_HEADER
# Getting Started

To build this project, you must have the following installed:

- git
- make
- docker (19.03 or higher)
- buildx (https://github.com/docker/buildx)

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
registry "$(REGISTRY)", username "$(USERNAME)", and a dynamic tag (e.g. $(REGISTRY_AND_USERNAME)/image:$(TAG)).
The registry and username can be overriden by exporting REGISTRY, and USERNAME
respectively.

endef

export HELP_MENU_HEADER

help: ## This help menu.
	@echo "$$HELP_MENU_HEADER"
	@grep -E '^[a-zA-Z%_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

# Build Abstractions

.PHONY: base
target-%: ## Builds the specified target defined in the Dockerfile. The build result will only remain in the build cache.
	@$(BUILD) \
		--target=$* \
		$(COMMON_ARGS) \
		$(TARGET_ARGS) .

local-%: ## Builds the specified target defined in the Dockerfile using the local output type. The build result will be output to the specified local destination.
	@$(MAKE) target-$* TARGET_ARGS="--output=type=local,dest=$(DEST) $(TARGET_ARGS)"

docker-%: ## Builds the specified target defined in the Dockerfile using the docker output type. The build result will be output to the specified local destination.
	@mkdir -p $(DEST)
	@$(MAKE) target-$* TARGET_ARGS="--output type=docker,dest=$(DEST)/$*.tar,name=$(REGISTRY_AND_USERNAME)/$*:$(TAG) $(TARGET_ARGS)"

hack-test-%: ## Runs the specied script in ./hack/test with well known environment variables.
	@./hack/test/$*.sh

# Generators

.PHONY: generate
generate: ## Generates source code from protobuf definitions.
	@$(MAKE) local-$@ DEST=./

.PHONY: docs
docs: ## Generates the documentation for machine config, and osctl.
	@rm -rf docs/osctl/*
	@$(MAKE) local-$@ DEST=./

# Local Artifacts

.PHONY: kernel
kernel: ## Outputs the kernel package contents (vmlinuz, and vmlinux) to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)
	@-rm -rf $(ARTIFACTS)/modules

.PHONY: initramfs
initramfs: ## Builds the compressed initramfs and outputs it to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) TARGET_ARGS="--allow security.insecure"

.PHONY: installer
installer: ## Builds the container image for the installer and outputs it to the artifact directory.
	@$(MAKE) docker-$@ DEST=$(ARTIFACTS) TARGET_ARGS="--allow security.insecure"
	@docker load < $(ARTIFACTS)/$@.tar

.PHONY: talos
talos: ## Builds the Talos container image and outputs it to the artifact directory.
	@$(MAKE) docker-$@ DEST=$(ARTIFACTS) TARGET_ARGS="--allow security.insecure"
	@mv $(ARTIFACTS)/$@.tar $(ARTIFACTS)/container.tar
	@docker load < $(ARTIFACTS)/container.tar

osctl-%:
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)

osctl: $(OSCTL_DEFAULT_TARGET) ## Builds the osctl binary for the local machine.

image-%: ## Builds the specified image. Valid options are aws, azure, digital-ocean, gcp, and vmware (e.g. image-aws)
	@docker run --rm -v /dev:/dev -v $(PWD)/$(ARTIFACTS):/out --privileged autonomy/installer:$(TAG) image --platform $*

images: image-aws image-azure image-digital-ocean image-gcp image-vmware ## Builds all known images (AWS, Azure, Digital Ocean, GCP, and VMware).

.PHONY: iso
iso: ## Builds the ISO and outputs it to the artifact directory.
	@docker run --rm -i -v $(PWD)/$(ARTIFACTS):/out autonomy/installer:$(TAG) iso

.PHONY: boot
boot: ## Creates a compressed tarball that includes vmlinuz and initramfs.xz. Note that these files must already be present in the artifacts directory.
	@tar  -C $(ARTIFACTS) -czf $(ARTIFACTS)/boot.tar.gz vmlinuz initramfs.xz

# Code Quality

.PHONY: fmt
fmt: ## Formats the source code.
	@docker run --rm -it -v $(PWD):/src -w /src golang:$(GO_VERSION) bash -c "export GO111MODULE=on; export GOPROXY=https://proxy.golang.org; cd /tmp && go mod init tmp && go get mvdan.cc/gofumpt/gofumports && cd - && gofumports -w -local github.com/talos-systems/talos ."

lint-%: ## Runs the specified linter. Valid options are go, protobuf, and markdown (e.g. lint-go).
	@$(MAKE) target-lint-$*

lint: ## Runs linters on go, protobuf, and markdown file types.
	@$(MAKE) lint-go lint-protobuf lint-markdown

# Tests

.PHONY: unit-tests
unit-tests: ## Performs unit tests.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS) TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-race
unit-tests-race: ## Performs unit tests with race detection enabled.
	@$(MAKE) target-$@

$(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(INTEGRATION_TEST_DEFAULT_TARGET) DEST=$(ARTIFACTS)

$(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64:
	@$(MAKE) local-$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET) DEST=$(ARTIFACTS)

$(ARTIFACTS)/sonobuoy:
	@mkdir -p $(ARTIFACTS)
	@curl -L -o /tmp/sonobuoy.tar.gz ${SONOBUOY_URL}
	@tar -xf /tmp/sonobuoy.tar.gz -C $(ARTIFACTS)

$(ARTIFACTS)/kubectl:
	@mkdir -p $(ARTIFACTS)
	@curl -L -o $(ARTIFACTS)/kubectl "$(KUBECTL_URL)"
	@chmod +x $(ARTIFACTS)/kubectl

e2e-%: $(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64 $(ARTIFACTS)/sonobuoy $(ARTIFACTS)/kubectl ## Runs the E2E test for the specified platform (e.g. e2e-docker).
	@$(MAKE) hack-test-$@ \
		PLATFORM=$* \
		TAG=$(TAG) \
		SHA=$(SHA) \
		IMAGE=$(REGISTRY_AND_USERNAME)/talos:$(TAG) \
		ARTIFACTS=$(ARTIFACTS) \
		OSCTL=$(PWD)/$(ARTIFACTS)/$(OSCTL_DEFAULT_TARGET)-amd64 \
		INTEGRATION_TEST=$(PWD)/$(ARTIFACTS)/$(INTEGRATION_TEST_DEFAULT_TARGET)-amd64 \
		KUBECTL=$(PWD)/$(ARTIFACTS)/kubectl \
		SONOBUOY=$(PWD)/$(ARTIFACTS)/sonobuoy

provision-tests: release-artifacts $(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64
	@$(MAKE) hack-test-$@ \
		TAG=$(TAG) \
		OSCTL=$(PWD)/$(ARTIFACTS)/$(OSCTL_DEFAULT_TARGET)-amd64 \
		INTEGRATION_TEST=$(PWD)/$(ARTIFACTS)/$(INTEGRATION_TEST_PROVISION_DEFAULT_TARGET)-amd64

# Assets for releases

.PHONY: $(ARTIFACTS)/$(TALOS_RELEASE)
$(ARTIFACTS)/$(TALOS_RELEASE): $(ARTIFACTS)/$(TALOS_RELEASE)/vmlinux $(ARTIFACTS)/$(TALOS_RELEASE)/initramfs.xz

# download release artifacts for specific version
$(ARTIFACTS)/$(TALOS_RELEASE)/%:
	@mkdir -p $(ARTIFACTS)/$(TALOS_RELEASE)/
	@curl -L -o "$(ARTIFACTS)/$(TALOS_RELEASE)/$*" "https://github.com/talos-systems/talos/releases/download/$(TALOS_RELEASE)/$*"

.PHONY: release-artifacts
release-artifacts:
	@for release in $(RELEASES); do \
		$(MAKE) $(ARTIFACTS)/$$release TALOS_RELEASE=$$release; \
	done

# Utilities

.PHONY: login
login: ## Logs in to the configured container registry.
ifeq ($(DOCKER_LOGIN_ENABLED), true)
	@docker login --username "$(DOCKER_USERNAME)" --password "$(DOCKER_PASSWORD)" $(REGISTRY)
endif

push: login ## Pushes the installer, and talos images to the configured container registry with the generated tag.
	@docker push $(REGISTRY_AND_USERNAME)/installer:$(TAG)
	@docker push $(REGISTRY_AND_USERNAME)/talos:$(TAG)

push-%: login ## Pushes the installer, and talos images to the configured container registry with the specified tag (e.g. push-latest).
	@docker tag $(REGISTRY_AND_USERNAME)/installer:$(TAG) $(REGISTRY_AND_USERNAME)/installer:$*
	@docker tag $(REGISTRY_AND_USERNAME)/talos:$(TAG) $(REGISTRY_AND_USERNAME)/talos:$*
	@docker push $(REGISTRY_AND_USERNAME)/installer:$*
	@docker push $(REGISTRY_AND_USERNAME)/talos:$*

.PHONY: clean
clean: ## Cleans up all artifacts.
	@-rm -rf $(ARTIFACTS)
