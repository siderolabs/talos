REGISTRY ?= docker.io
USERNAME ?= autonomy
SHA ?= $(shell git describe --match=none --always --abbrev=8 --dirty)
TAG ?= $(shell git describe --tag --always --dirty)
BRANCH ?= $(shell git rev-parse --abbrev-ref HEAD)
REGISTRY_AND_USERNAME := $(REGISTRY)/$(USERNAME)

ARTIFACTS := _out
TOOLS ?= autonomy/tools:8fdb32d
GO_VERSION ?= 1.13
OPERATING_SYSTEM := $(shell uname -s | tr "[:upper:]" "[:lower:]")
OSCTL_DEFAULT_TARGET := osctl-$(OPERATING_SYSTEM)
OSCTL_COMMAND := $(ARTIFACTS)/osctl-$(OPERATING_SYSTEM)-amd64
TESTPKGS ?= ./...

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
COMMON_ARGS += --build-arg=IMAGES=$(ARTIFACTS)/images
COMMON_ARGS += --build-arg=TESTPKGS=$(TESTPKGS)

all: help

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

> Note: The security.insecure entitlement is only required, and used by the unit-tests target.

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
	@TAG=$(TAG) SHA=$(SHA) ARTIFACTS=$(ARTIFACTS) ./hack/test/$*.sh

# Generators

.PHONY: generate
generate: ## Generates source code from protobuf definitions.
	@$(MAKE) local-$@ DEST=./

.PHONY: docs
docs: ## Generates the documentation for machine config, and osctl.
	@rm -rf docs/osctl/*
	@$(MAKE) local-$@ DEST=./

# Apps

apid: ## Builds the apid container image. The build result will be output to the specified local destination.
	@$(MAKE) docker-$@ DEST=./$(ARTIFACTS)/images

machined: ## Builds machined. The build result will only remain in the build cache.
	@$(MAKE) target-$@

networkd: ## Builds the networkd container image. The build result will be output to the specified local destination.
	@$(MAKE) docker-$@ DEST=./$(ARTIFACTS)/images

ntpd: ## Builds the ntpd container image. The build result will be output to the specified local destination.
	@$(MAKE) docker-$@ DEST=./$(ARTIFACTS)/images

osd: ## Builds the osd container image. The build result will be output to the specified local destination.
	@$(MAKE) docker-$@ DEST=./$(ARTIFACTS)/images

trustd: ## Builds the trustd container image. The build result will be output to the specified local destination.
	@$(MAKE) docker-$@ DEST=./$(ARTIFACTS)/images

apps: apid machined networkd ntpd osd trustd ## Builds all apps (apid, machined, networkd, ntpd, osd, and trustd).

# Local Artifacts

.PHONY: kernel
kernel: ## Outputs the kernel package contents (vmlinuz, and vmlinux) to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)
	@-rm -rf $(ARTIFACTS)/modules

.PHONY: initramfs
initramfs: ## Builds the compressed initramfs and outputs it to the artifact directory.
	@$(MAKE) local-$@ DEST=$(ARTIFACTS)

.PHONY: installer
installer: ## Builds the container image for the installer and outputs it to the artifact directory.
	@$(MAKE) docker-$@ DEST=$(ARTIFACTS)
	@docker load < $(ARTIFACTS)/$@.tar

.PHONY: talos
talos: ## Builds the Talos container image and outputs it to the artifact directory.
	@$(MAKE) docker-$@ DEST=$(ARTIFACTS)
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
unit-tests: apps ## Performs unit tests.
	@$(MAKE) local-$@ DEST=./ TARGET_ARGS="--allow security.insecure"

.PHONY: unit-tests-race
unit-tests-race: ## Performs unit tests with race detection enabled.
	@$(MAKE) local-$@ DEST=./

.PHONY: integration-test
integration-test: ## Runs the CLI and API integration tests against a running cluster.
	@$(MAKE) local-$@ DEST=./bin

.PHONY: basic-integration
basic-integration: ## Runs the basic integration test.
	@TAG=$(TAG) SHA=$(SHA) ARTIFACTS=$(ARTIFACTS) go run ./internal/test-framework/main.go $@ --artifacts=$(ARTIFACTS)

.PHONY: e2e-integration
e2e-integration: ## Runs the E2E integration for the specified cloud provider.
	@$(MAKE) hack-test-$@

push-image-%: ## Pushes a VM image into the specified cloud provider. Valid options are aws, azure, and gcp (e.g. push-image-aws).
	@$(MAKE) hack-test-$*-setup

.PHONY: capi
capi: ## Deploys Cluster API to the basic integration cluster.
	@$(MAKE) hack-test-$@

# Utilities

.PHONY: login
login: ## Logs in to the configured container registry.
	@docker login --username "$(DOCKER_USERNAME)" --password "$(DOCKER_PASSWORD)" $(REGISTRY)

push-%: login ## Pushes the installer, and talos images to the configured container registry with the specified tag (e.g. push-latest).
	@docker push autonomy/installer:$(TAG)
	@docker push autonomy/talos:$(TAG)
ifeq ($(BRANCH),master)
	@docker tag autonomy/installer:$(TAG) autonomy/installer:$*
	@docker tag autonomy/talos:$(TAG) autonomy/talos:$*
	@docker push autonomy/installer:$*
	@docker push autonomy/talos:$*
endif

.PHONY: clean
clean: ## Cleans up all artifacts.
	@-rm -rf $(ARTIFACTS) coverage.txt
