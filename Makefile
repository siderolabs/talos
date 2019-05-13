SHA = $(shell gitmeta git sha)
TAG = $(shell gitmeta image tag)

KERNEL_IMAGE ?= autonomy/kernel:1f83e85
TOOLCHAIN_IMAGE ?= autonomy/toolchain:c28db88
ROOTFS_IMAGE ?= autonomy/rootfs-base:c28db88
INITRAMFS_IMAGE ?= autonomy/initramfs-base:c28db88

# TODO(andrewrynhard): Move this logic to a shell script.
BUILDKIT_VERSION ?= v0.5.0
KUBECTL_VERSION ?= v1.14.1
BUILDKIT_IMAGE ?= moby/buildkit:$(BUILDKIT_VERSION)
BUILDKIT_HOST ?= tcp://0.0.0.0:1234
BUILDKIT_CONTAINER_NAME ?= talos-buildkit
BUILDKIT_CONTAINER_STOPPED := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=exited --format='{{.Names}}' 2>/dev/null)
BUILDKIT_CONTAINER_RUNNING := $(shell docker ps --filter name=$(BUILDKIT_CONTAINER_NAME) --filter status=running --format='{{.Names}}' 2>/dev/null)

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).linux-amd64.tar.gz
BUILDKIT_CACHE ?= -v $(HOME)/.buildkit:/var/lib/buildkit
endif
ifeq ($(UNAME_S),Darwin)
BUILDCTL_ARCHIVE := https://github.com/moby/buildkit/releases/download/$(BUILDKIT_VERSION)/buildkit-$(BUILDKIT_VERSION).darwin-amd64.tar.gz
BUILDKIT_CACHE ?= ""
endif

ifeq ($(UNAME_S),Linux)
KUBECTL_ARCHIVE := https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl
endif
ifeq ($(UNAME_S),Darwin)
KUBECTL_ARCHIVE := https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/darwin/amd64/kubectl
endif

BINDIR ?= ./bin
CONFORM_VERSION ?= 57c9dbd

COMMON_ARGS = --progress=plain
COMMON_ARGS += --frontend=dockerfile.v0
COMMON_ARGS += --local context=.
COMMON_ARGS += --local dockerfile=.
COMMON_ARGS += --opt build-arg:KERNEL_IMAGE=$(KERNEL_IMAGE)
COMMON_ARGS += --opt build-arg:TOOLCHAIN_IMAGE=$(TOOLCHAIN_IMAGE)
COMMON_ARGS += --opt build-arg:ROOTFS_IMAGE=$(ROOTFS_IMAGE)
COMMON_ARGS += --opt build-arg:INITRAMFS_IMAGE=$(INITRAMFS_IMAGE)
COMMON_ARGS += --opt build-arg:SHA=$(SHA)
COMMON_ARGS += --opt build-arg:TAG=$(TAG)

DOCKER_ARGS ?=
# to allow tests to run containerd
DOCKER_TEST_ARGS = --security-opt seccomp:unconfined --privileged -v /var/lib/containerd/

all: ci drone

.PHONY: drone
drone: rootfs initramfs kernel installer talos

.PHONY: ci
ci: builddeps buildkitd


.PHONY: builddeps
builddeps: gitmeta buildctl

gitmeta:
	GO111MODULE=off go get github.com/talos-systems/gitmeta

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
		$(BUILDKIT_CACHE) \
		$(BUILDKIT_IMAGE) \
		--addr $(BUILDKIT_HOST)
	@echo "Wait for buildkitd to become available"
	@sleep 5
endif
endif

.PHONY: binaries
binaries: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
    --output type=local,dest=build \
		--opt target=$@ \
		$(COMMON_ARGS)

base: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=build/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
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
rootfs: buildkitd binaries osd trustd proxyd ntpd udevd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
    --output type=local,dest=build \
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

.PHONY: proto
proto: buildkitd
	$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
    --output type=local,dest=./ \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: talos-gce
talos-gce:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out --privileged $(DOCKER_ARGS) autonomy/installer:$(TAG) install -n disk -r -p googlecloud -u none
	@tar -C $(PWD)/build -Sczf $(PWD)/build/$@.tar.gz disk.raw
	@rm $(PWD)/build/disk.raw

.PHONY: talos-iso
talos-iso:
	@docker run --rm -i -v $(PWD)/build:/out autonomy/installer:$(TAG) iso

.PHONY: talos-aws
talos-aws:
	@docker run \
		--rm \
		-i \
		-e AWS_ACCESS_KEY_ID=$(AWS_ACCESS_KEY_ID) \
		-e AWS_SECRET_ACCESS_KEY=$(AWS_SECRET_ACCESS_KEY) \
		-e AWS_DEFAULT_REGION=$(AWS_DEFAULT_REGION) \
		autonomy/installer:$(TAG) ami -var regions=${AWS_PUBLISH_REGIONS} -var visibility=all

.PHONY: talos-raw
talos-raw:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out --privileged $(DOCKER_ARGS) autonomy/installer:$(TAG) install -n rootfs -r -b

.PHONY: talos
talos: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=build/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

.PHONY: integration
integration:
	@KUBERNETES_VERSION=v1.14.1 ./hack/test/integration.sh

.PHONY: test
test: buildkitd
	@mkdir -p build
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=/tmp/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)
	@docker load < /tmp/$@.tar
	@trap "rm -rf ./.artifacts" EXIT; mkdir -p ./.artifacts && \
		docker run -i --rm $(DOCKER_TEST_ARGS) -v $(PWD)/.artifacts:/src/artifacts autonomy/$@:$(TAG) /bin/test.sh && \
		cp ./.artifacts/coverage.txt coverage.txt

.PHONY: dev-test
dev-test:
	@docker run -i --rm $(DOCKER_TEST_ARGS) \
		-v $(PWD)/internal:/src/internal:ro \
		-v $(PWD)/pkg:/src/pkg:ro \
		-v $(PWD)/cmd:/src/cmd:ro \
		autonomy/test:$(TAG) \
		go test -v ./...

.PHONY: lint
lint: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: osctl-linux-amd64
osctl-linux-amd64: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
    --output type=local,dest=build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: osctl-darwin-amd64
osctl-darwin-amd64: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
    --output type=local,dest=build \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: udevd
udevd: buildkitd
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--output type=docker,dest=images/$@.tar,name=docker.io/autonomy/$@:$(TAG) \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: osd
osd: buildkitd images
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

.PHONY: proxyd
proxyd: buildkitd images
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

images:
	@mkdir images

.PHONY: login
login:
	@docker login --username "$(DOCKER_USERNAME)" --password "$(DOCKER_PASSWORD)"

.PHONY: push
push:
	@docker tag autonomy/installer:$(TAG) autonomy/installer:latest
	@docker push autonomy/installer:$(TAG)
	@docker push autonomy/installer:latest
	@docker tag autonomy/talos:$(TAG) autonomy/talos:latest
	@docker push autonomy/talos:$(TAG)
	@docker push autonomy/talos:latest

.PHONY: clean
clean:
	@-rm -rf build images vendor
