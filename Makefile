TOOLS ?= autonomy/tools:8fdb32d

KUBECTL_VERSION ?= v1.17.0
GO_VERSION ?= 1.13

UNAME_S := $(shell uname -s)

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

ifeq ($(UNAME_S),Linux)
OSCTL_DEFAULT_TARGET := osctl-linux
OSCTL_COMMAND := build/osctl-linux-amd64
endif
ifeq ($(UNAME_S),Darwin)
OSCTL_DEFAULT_TARGET := osctl-darwin
OSCTL_COMMAND := build/osctl-darwin-amd64
endif

BINDIR ?= ./bin

REGISTRY ?= docker.io
USERNAME ?= autonomy
SHA ?= $(shell $(BINDIR)/gitmeta git sha)
TAG ?= $(shell $(BINDIR)/gitmeta image tag)
BRANCH ?= $(shell $(BINDIR)/gitmeta git branch)
REGISTRY_AND_USERNAME := $(REGISTRY)/$(USERNAME)

PLATFORM ?= linux/amd64
PROGRESS ?= auto
PUSH ?= false

BUILD := docker buildx build
COMMON_ARGS := --file=Dockerfile
COMMON_ARGS += --progress=$(PROGRESS)
COMMON_ARGS += --platform=$(PLATFORM)
COMMON_ARGS += --push=$(PUSH)
COMMON_ARGS += --build-arg=TOOLS=$(TOOLS)
COMMON_ARGS += --build-arg=SHA=$(SHA)
COMMON_ARGS += --build-arg=TAG=$(TAG)
COMMON_ARGS += --build-arg=GO_VERSION=$(GO_VERSION)
COMMON_ARGS += .

DOCKER_ARGS ?=

TESTPKGS ?= ./...

all: ci rootfs initramfs kernel osctl-linux osctl-darwin installer container

.PHONY: ci
ci: builddeps

.PHONY: builddeps
builddeps: gitmeta

gitmeta: $(BINDIR)/gitmeta

$(BINDIR)/gitmeta:
	@mkdir -p $(BINDIR)
	@curl -L $(GITMETA) -o $(BINDIR)/gitmeta
	@chmod +x $(BINDIR)/gitmeta

kubectl: $(BINDIR)/kubectl

$(BINDIR)/kubectl:
	@mkdir -p $(BINDIR)
	@curl -L -o $(BINDIR)/kubectl $(KUBECTL_ARCHIVE)
	@chmod +x $(BINDIR)/kubectl

base:
	@$(BUILD) \
		--output type=docker,dest=build/$@.tar,name=$(REGISTRY_AND_USERNAME)/$@:$(TAG) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: generate
generate:
	$(BUILD) \
		--output type=local,dest=./ \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: docs
docs: $(OSCTL_DEFAULT_TARGET)
	$(BUILD) \
		--output type=local,dest=./ \
		--target=$@ \
		$(COMMON_ARGS)
	@env HOME=/home/user $(OSCTL_COMMAND) docs docs/osctl

.PHONY: kernel
kernel:
	@$(BUILD) \
		--output type=local,dest=build \
		--target=$@ \
		$(COMMON_ARGS)
	@-rm -rf ./build/modules

.PHONY: initramfs
initramfs:
	@$(BUILD) \
		--output type=local,dest=build \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: squashfs
squashfs: osd trustd ntpd networkd apid
	@$(BUILD) \
		--output type=local,dest=build \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: rootfs
rootfs: osd trustd ntpd networkd apid
	@$(BUILD) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: installer
installer:
	@mkdir -p build
	@$(BUILD) \
		--output type=docker,dest=build/$@.tar,name=$(REGISTRY_AND_USERNAME)/$@:$(TAG) \
		--target=$@ \
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
	@gzip -f $(PWD)/build/digital-ocean.raw

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

.PHONY: image-vmware
image-vmware:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out \
		--privileged $(DOCKER_ARGS) \
		autonomy/installer:$(TAG) \
		install \
		-r \
		-p vmware \
		-u guestinfo \
		-e console=tty0 \
		-e earlyprintk=ttyS0,115200
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out \
		--privileged $(DOCKER_ARGS) \
		autonomy/installer:$(TAG) \
		ova
	@rm -rf $(PWD)/build/talos.raw

.PHONY: push-image-aws
push-image-aws:
	@TAG=$(TAG) SHA=$(SHA) ./hack/test/aws-setup.sh

.PHONY: push-image-azure
push-image-azure:
	@TAG=$(TAG) ./hack/test/azure-setup.sh

.PHONY: push-image-gcp
push-image-gcp:
	@TAG=$(TAG) SHA=$(SHA) ./hack/test/gcp-setup.sh

.PHONY: image-test
image-test:
	@docker run --rm -v /dev:/dev -v /tmp:/out --privileged $(DOCKER_ARGS) autonomy/installer:$(TAG) install -n test -r -p test -u none

.PHONY: iso
iso:
	@docker run --rm -i -v $(PWD)/build:/out autonomy/installer:$(TAG) iso

.PHONY: container
container:
	@$(BUILD) \
		--output type=docker,dest=build/$@.tar,name=$(REGISTRY_AND_USERNAME)/talos:$(TAG) \
		--target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

.PHONY: basic-integration
basic-integration: gitmeta
	@TAG=$(TAG) SHA=$(SHA) go run ./internal/test-framework/main.go basic-integration

.PHONY: capi
capi:
	@TAG=$(TAG) ./hack/test/$@.sh

.PHONY: e2e-integration
e2e-integration:
	@TAG=$(TAG) SHA=$(SHA) ./hack/test/$@.sh

.PHONY: unit-tests
unit-tests:
	@$(BUILD) \
		--target=$@ \
		--output type=local,dest=./ \
		--build-arg=TESTPKGS=$(TESTPKGS) \
		--allow security.insecure \
		$(COMMON_ARGS)

.PHONY: unit-tests-race
unit-tests-race:
	@$(BUILD) \
		--target=$@ \
		--build-arg=TESTPKGS=$(TESTPKGS) \
		$(COMMON_ARGS)

.PHONY: integration-test
integration-test:
	@$(BUILD) \
		--output type=local,dest=bin \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: fmt
fmt:
	@docker run --rm -it -v $(PWD):/src -w /src golang:$(GO_VERSION) bash -c "export GO111MODULE=on; export GOPROXY=https://proxy.golang.org; cd /tmp && go mod init tmp && go get mvdan.cc/gofumpt/gofumports && cd - && gofumports -w -local github.com/talos-systems/talos ."

.PHONY: lint
lint:
	@$(BUILD) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: protolint
protolint:
	@$(BUILD) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: markdownlint
markdownlint:
	@$(BUILD) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: osctl-linux
osctl-linux:
	@$(BUILD) \
		--output type=local,dest=build \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: osctl-darwin
osctl-darwin:
	@$(BUILD) \
		--output type=local,dest=build \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: machined
machined: images
	@$(BUILD) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: osd
osd: images
	@$(BUILD) \
		--output type=docker,dest=images/$@.tar,name=$(REGISTRY_AND_USERNAME)/$@:$(TAG) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: apid
apid: images
	@$(BUILD) \
		--output type=docker,dest=images/$@.tar,name=$(REGISTRY_AND_USERNAME)/$@:$(TAG) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: trustd
trustd: images
	@$(BUILD) \
		--output type=docker,dest=images/$@.tar,name=$(REGISTRY_AND_USERNAME)/$@:$(TAG) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: ntpd
ntpd: images
	@$(BUILD) \
		--output type=docker,dest=images/$@.tar,name=$(REGISTRY_AND_USERNAME)/$@:$(TAG) \
		--target=$@ \
		$(COMMON_ARGS)

.PHONY: networkd
networkd: images
	@$(BUILD) \
		--output type=docker,dest=images/$@.tar,name=$(REGISTRY_AND_USERNAME)/$@:$(TAG) \
		--target=$@ \
		$(COMMON_ARGS)

images:
	@mkdir -p images

.PHONY: login
login:
	@docker login --username "$(DOCKER_USERNAME)" --password "$(DOCKER_PASSWORD)"

push-%: gitmeta login
	@docker push autonomy/installer:$(TAG)
	@docker push autonomy/talos:$(TAG)
ifeq ($(BRANCH),master)
	@docker tag autonomy/installer:$(TAG) autonomy/installer:$*
	@docker tag autonomy/talos:$(TAG) autonomy/talos:$*
	@docker push autonomy/installer:$*
	@docker push autonomy/talos:$*
endif

.PHONY: clean
clean:
	@-rm -rf build images vendor
