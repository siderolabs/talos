ARCH ?= amd64

SHA = $(shell gitmeta git sha)
TAG = $(shell gitmeta image tag)-$(ARCH)

TOOLS_REGISTRY := docker.io
TOOLS_USERNAME := andrewrynhard
TOOLS_REPO := tools
TOOLS_TAG := ee30671
TOOLS := $(TOOLS_REGISTRY)/$(TOOLS_USERNAME)/$(TOOLS_REPO):$(TOOLS_TAG)

PROGRESS := auto
PLATFORM := linux/$(ARCH)
PUSH := false
CACHE_FROM := type=local,src=./out/cache
CACHE_TO := type=local,dest=./out/cache

BUILD = docker buildx build
COMMON_ARGS  = --progress=$(PROGRESS)
COMMON_ARGS += --platform=$(PLATFORM)
COMMON_ARGS += --build-arg=TOOLS=$(TOOLS)
COMMON_ARGS += --build-arg=SHA=$(SHA)
COMMON_ARGS += --build-arg=TAG=$(TAG)
COMMON_ARGS += --file=./Dockerfile
# COMMON_ARGS += --cache-from=$(CACHE_FROM)
# COMMON_ARGS += --cache-to=$(CACHE_TO)
EXTRA_ARGS ?=

OUT_PATH = ./out/linux_$(ARCH)
IMAGE_PATH = $(OUT_PATH)/images

all: kernel initramfs rootfs talos osctl-linux installer

.PHONY: help
help: ## This help menu.
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

.PHONY: generate
generate: ## Generates the source code from protobuf definitions.
	@$(BUILD) $(COMMON_ARGS) $(EXTRA_ARGS) --target=$@ --output=type=local,dest=. .

.PHONY: base
base: ## Builds a base image that is used for all builds.
	@$(BUILD) $(COMMON_ARGS) $(EXTRA_ARGS) --target=$@ .

.PHONY: test
test: ## Performs unit and functional tests.
	@$(BUILD) $(COMMON_ARGS) --target=$@ --tag=autonomy/$@:$(TAG) --load .
	@trap "rm -rf ./.artifacts" EXIT; mkdir -p ./.artifacts \
		&& docker run -i --rm --security-opt seccomp:unconfined --privileged -v /var/lib/containerd/ -v $(PWD)/.artifacts:/src/artifacts autonomy/$@:$(TAG) /toolchain/bin/test.sh \
		&& cp ./.artifacts/coverage.txt coverage.txt

.PHONY: basic-integration
basic-integration:
	@KUBERNETES_VERSION=v1.15.0 ./hack/test/$@.sh

.PHONY: e2e
e2e-integration:
	@KUBERNETES_VERSION=v1.15.0 ./hack/test/$@.sh

.PHONY: lint
lint: ## Runs linting on the source.
	@$(BUILD) $(COMMON_ARGS) --target=$@ .

.PHONY: kernel
kernel: ## Copies the kernel to $(OUT_PATH).
	@$(BUILD) $(COMMON_ARGS) --target=$@ --output=type=local,dest=$(OUT_PATH) .

.PHONY: initramfs
initramfs: ## Builds the compressed initramfs and outputs it to ./out/initramfs.xz.
	@$(BUILD) $(COMMON_ARGS) --target=$@ --output=type=local,dest=$(OUT_PATH) .

.PHONY: rootfs
rootfs: ntpd osd proxyd trustd ## Builds the compressed rootfs and outputs it to ./out/rootfs.tar.gz.
	@$(BUILD) $(COMMON_ARGS) --target=$@ --output=type=local,dest=$(OUT_PATH) .

.PHONY: integration
integration:
	@docker load <$(OUT_PATH)/talos-$(ARCH).tar
	@KUBERNETES_VERSION=v1.14.1 ./hack/test/integration.sh

.PHONY: e2e
e2e:
	@docker load <$(OUT_PATH)/talos-$(ARCH).tar
	@KUBERNETES_VERSION=v1.14.1 ./hack/test/e2e.sh

talos: ## Builds the container image for Talos.
	@$(BUILD) $(COMMON_ARGS) --target=$@ --tag=autonomy/$@:$(TAG) --output=type=docker,dest=$(OUT_PATH)/$@-$(ARCH).tar .

.PHONY: installer
installer: ## Builds the container image for the Talos installer.
	@$(BUILD) $(COMMON_ARGS) --target=$@ --tag=autonomy/$@:$(TAG) --output=type=docker,dest=$(OUT_PATH)/$@-$(ARCH).tar .

.PHONY: osctl-linux
osctl-linux: ## Builds the osctl binary for linux.
	@$(BUILD) $(COMMON_ARGS) --target=$@ --output=type=local,dest=$(OUT_PATH) .

.PHONY: osctl-darwin-amd64
osctl-darwin-amd64: ## Builds the osctl binary for darwin.
	@$(BUILD) $(COMMON_ARGS) --target=$@ --output=type=local,dest=$(OUT_PATH) .

.PHONY: init
init: ## Builds the init binary.
	@$(BUILD) $(COMMON_ARGS) --target=$@ .

.PHONY: ntpd
ntpd: $(IMAGE_PATH)/ntpd.tar ## Builds the ntpd container and outputs it to ./out/images/ntpd.tar.
$(IMAGE_PATH)/ntpd.tar:
	@$(BUILD) $(COMMON_ARGS) --target=ntpd --tag=autonomy/ntpd:$(TAG) --output=type=docker,dest=$(IMAGE_PATH)/ntpd.tar .

.PHONY: osd
osd: $(IMAGE_PATH)/osd.tar ## Builds the osd container and outputs it to ./out/images/osd.tar.
$(IMAGE_PATH)/osd.tar:
	@$(BUILD) $(COMMON_ARGS) --target=osd --tag=autonomy/osd:$(TAG) --output=type=docker,dest=./$(IMAGE_PATH)/osd.tar .

.PHONY: proxyd
proxyd: $(IMAGE_PATH)/proxyd.tar ## Builds the proxyd container and outputs it to ./out/images/proxyd.tar.
$(IMAGE_PATH)/proxyd.tar:
	@$(BUILD) $(COMMON_ARGS) --target=proxyd --tag=autonomy/proxyd:$(TAG) --output=type=docker,dest=./$(IMAGE_PATH)/proxyd.tar .

.PHONY: trustd
trustd: $(IMAGE_PATH)/trustd.tar ## Builds the trustd container and outputs it to ./out/images/trustd.tar.
$(IMAGE_PATH)/trustd.tar:
	@$(BUILD) $(COMMON_ARGS) --target=trustd --tag=autonomy/trustd:$(TAG) --output=type=docker,dest=./$(IMAGE_PATH)/trustd.tar .

clean:
	-@rm -rf ./out
