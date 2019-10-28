TOOLS ?= autonomy/tools:8fdb32d

# TODO(andrewrynhard): Move this logic to a shell script.
BUILDKIT_VERSION ?= v0.6.0
KUBECTL_VERSION ?= v1.16.0
GO_VERSION ?= 1.13
BUILDKIT_IMAGE ?= moby/buildkit:$(BUILDKIT_VERSION)
BUILDKIT_HOST ?= tcp://0.0.0.0:1234
BUILDKIT_CONTAINER_NAME ?= talos-buildkit
BUILDKIT_CONTAINER_STOPPED := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=exited --format='{{.Names}}' 2>/dev/null)
BUILDKIT_CONTAINER_RUNNING := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=running --format='{{.Names}}' 2>/dev/null)

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).linux-amd64.tar.gz
endif
ifeq ($(UNAME_S),Darwin)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).darwin-amd64.tar.gz
endif

ifeq ($(UNAME_S),Linux)
KUBECTL_ARCHIVE := https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl
endif
ifeq ($(UNAME_S),Darwin)
KUBECTL_ARCHIVE := https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/darwin/amd64/kubectl
endif

ifeq ($(UNAME_S),Linux)
GITMETA := https://github.com/talos-systems/gitmeta/releases/download/v0.1.0-alpha.3/gitmeta-linux-amd64
endif
ifeq ($(UNAME_S),Darwin)
GITMETA := https://github.com/talos-systems/gitmeta/releases/download/v0.1.0-alpha.3/gitmeta-darwin-amd64
endif

BINDIR ?= ./bin
CONFORM_VERSION ?= 57c9dbd

SHA ?= $(shell $(BINDIR)/gitmeta git sha)
TAG ?= $(shell $(BINDIR)/gitmeta image tag)
BRANCH ?= $(shell $(BINDIR)/gitmeta git branch)

COMMON_ARGS = --progress=plain
COMMON_ARGS += --frontend=dockerfile.v0
COMMON_ARGS += --allow security.insecure
COMMON_ARGS += --local context=.
COMMON_ARGS += --local dockerfile=.
COMMON_ARGS += --opt build-arg:TOOLS=$(TOOLS)
COMMON_ARGS += --opt build-arg:SHA=$(SHA)
COMMON_ARGS += --opt build-arg:TAG=$(TAG)
COMMON_ARGS += --opt build-arg:GO_VERSION=$(GO_VERSION)

DOCKER_ARGS ?=

TESTPKGS ?= ./...

all: ci rootfs initramfs kernel osctl-linux osctl-darwin installer container

.PHONY: ci
ci: builddeps buildkitd

.PHONY: builddeps
builddeps: gitmeta buildctl

gitmeta: $(BINDIR)/gitmeta

$(BINDIR)/gitmeta:
	@mkdir -p $(BINDIR)
	@curl -L $(GITMETA) -o $(BINDIR)/gitmeta
	@chmod +x $(BINDIR)/gitmeta

buildctl: $(BINDIR)/buildctl

$(BINDIR)/buildctl:
	@mkdir -p $(BINDIR)
	@curl -L $(BUILDCTL_ARCHIVE) | tar -zxf - -C $(BINDIR) --strip-components 1 bin/buildctl

kubectl: $(BINDIR)/kubectl

$(BINDIR)/kubectl:
	@mkdir -p $(BINDIR)
	@curl -L -o $(BINDIR)/kubectl $(KUBECTL_ARCHIVE)
	@chmod +x $(BINDIR)/kubectl

.PHONY: buildkitd
buildkitd:
ifeq (tcp://0.0.0.0:1234,$(findstring tcp://0.0.0.0:1234,$(BUILDKIT_HOST)))
ifeq ($(BUILDKIT_CONTAINER_STOPPED),$(BUILDKIT_CONTAINER_NAME))
	@echo "Removing exited talos-buildkit container"
	@docker rm $(BUILDKIT_CONTAINER_NAME)
endif
ifneq ($(BUILDKIT_CONTAINER_RUNNING),$(BUILDKIT_CONTAINER_NAME))
	@echo "Starting talos-buildkit container"
	@docker run \
		--name $(BUILDKIT_CONTAINER_NAME) \
		-d \
		--privileged \
		-p 1234:1234 \
		$(BUILDKIT_IMAGE) \
		--addr $(BUILDKIT_HOST) \
		--allow-insecure-entitlement security.insecure
	@echo "Wait for buildkitd to become available"
	@sleep 5
endif
endif

base: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=build/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: generate
generate: buildkitd
	$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=local,dest=./ \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: kernel
kernel: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=local,dest=build \
		--opt target=$@ \
		$(COMMON_ARGS)
	@-rm -rf ./build/modules

.PHONY: initramfs
initramfs: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=local,dest=build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: rootfs
rootfs: buildkitd osd trustd ntpd networkd apid
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: installer
installer: buildkitd
	@mkdir -p build
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=build/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

.PHONY: image-aws
image-aws:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out \
		--privileged $(DOCKER_ARGS) \
		autonomy/installer:$(TAG) \
		install \
		-n aws \
		-r \
		-p aws \
		-u none \
		-e console=tty1 \
		-e console=ttyS0
	@tar -C $(PWD)/build -czf $(PWD)/build/aws.tar.gz aws.raw
	@rm -rf $(PWD)/build/aws.raw

.PHONY: image-azure
image-azure:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out \
		--privileged $(DOCKER_ARGS) \
		autonomy/installer:$(TAG) \
		install \
		-n azure \
		-r \
		-p azure \
		-u none \
		-e console=ttyS0,115200n8 \
		-e earlyprintk=ttyS0,115200 \
		-e rootdelay=300
	@docker run --rm -v $(PWD)/build:/out $(DOCKER_ARGS) \
		--entrypoint qemu-img \
		autonomy/installer:$(TAG) \
		convert \
		-f raw \
		-o subformat=fixed,force_size \
		-O vpc /out/azure.raw /out/azure.vhd
	@tar -C $(PWD)/build -czf $(PWD)/build/azure.tar.gz azure.vhd
	@rm -rf $(PWD)/build/azure.raw $(PWD)/build/azure.vhd

.PHONY: image-digital-ocean
image-digital-ocean:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out \
		--privileged $(DOCKER_ARGS) \
		autonomy/installer:$(TAG) \
		install \
		-n digital-ocean \
		-r \
		-p digital-ocean \
		-u none \
		-e console=ttyS0
	@gzip $(PWD)/build/digital-ocean.raw

.PHONY: image-gcp
image-gcp:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out \
		--privileged $(DOCKER_ARGS) \
		autonomy/installer:$(TAG) \
		install \
		-n disk \
		-r \
		-p gcp \
		-u none \
		-e console=ttyS0
	@tar -C $(PWD)/build -czf $(PWD)/build/gcp.tar.gz disk.raw
	@rm -rf $(PWD)/build/disk.raw

.PHONY: push-image-aws
push-image-aws:
	@TAG=$(TAG) ./hack/test/aws-setup.sh

.PHONY: push-image-azure
push-image-azure:
	@TAG=$(TAG) ./hack/test/azure-setup.sh

.PHONY: push-image-gcp
push-image-gcp:
	@TAG=$(TAG) ./hack/test/gcp-setup.sh

.PHONY: image-test
image-test:
	@docker run --rm -v /dev:/dev -v /tmp:/out --privileged $(DOCKER_ARGS) autonomy/installer:$(TAG) install -n test -r -p test -u none

.PHONY: iso
iso:
	@docker run --rm -i -v $(PWD)/build:/out autonomy/installer:$(TAG) iso

.PHONY: container
container: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=build/$@.tar,name=docker.io/autonomy/talos:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

.PHONY: basic-integration
basic-integration:
	@TAG=$(TAG) ./hack/test/$@.sh

.PHONY: capi
capi:
	@TAG=$(TAG) ./hack/test/$@.sh

.PHONY: e2e-integration
e2e-integration:
	@TAG=$(TAG) ./hack/test/$@.sh

.PHONY: unit-tests
unit-tests: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		--output type=local,dest=./ \
		--opt build-arg:TESTPKGS=$(TESTPKGS) \
		$(COMMON_ARGS)

.PHONY: unit-tests-race
unit-tests-race: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		--opt build-arg:TESTPKGS=$(TESTPKGS) \
		$(COMMON_ARGS)

.PHONY: fmt
fmt:
	@docker run --rm -it -v $(PWD):/src -w /src golang:$(GO_VERSION) bash -c "export GO111MODULE=on; export GOPROXY=https://proxy.golang.org; cd /tmp && go mod init tmp && go get mvdan.cc/gofumpt/gofumports && cd - && gofumports -w -local github.com/talos-systems/talos ."

.PHONY: lint
lint: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: protolint
protolint: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: markdownlint
markdownlint: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: osctl-linux
osctl-linux: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=local,dest=build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: osctl-darwin
osctl-darwin: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=local,dest=build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: machined
machined: buildkitd images
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: osd
osd: buildkitd images
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=images/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: apid
apid: buildkitd images
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=images/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: trustd
trustd: buildkitd images
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=images/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: ntpd
ntpd: buildkitd images
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=images/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: networkd
networkd: buildkitd images
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=images/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)

images:
	@mkdir -p images

.PHONY: login
login:
	@docker login --username "$(DOCKER_USERNAME)" --password "$(DOCKER_PASSWORD)"

.PHONY: push
push: gitmeta login
	@docker push autonomy/installer:$(TAG)
	@docker push autonomy/talos:$(TAG)
ifeq ($(BRANCH),master)
	@docker tag autonomy/installer:$(TAG) autonomy/installer:latest
	@docker tag autonomy/talos:$(TAG) autonomy/talos:latest
	@docker push autonomy/installer:latest
	@docker push autonomy/talos:latest
endif

.PHONY: clean
clean:
	@-rm -rf build images vendor
