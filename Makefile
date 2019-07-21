TOOLS ?= autonomy/tools:b473afb

# TODO(andrewrynhard): Move this logic to a shell script.
BUILDKIT_VERSION ?= master@sha256:455f06ede03149051ce2734d9639c28aed1b6e8b8a0c607cb813e29b469a07d6
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
BUILDKIT_CACHE ?=
endif

ifeq ($(UNAME_S),Linux)
KUBECTL_ARCHIVE := https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/linux/amd64/kubectl
endif
ifeq ($(UNAME_S),Darwin)
KUBECTL_ARCHIVE := https://storage.googleapis.com/kubernetes-release/release/$(KUBECTL_VERSION)/bin/darwin/amd64/kubectl
endif

ifeq ($(UNAME_S),Linux)
GITMETA := https://github.com/talos-systems/gitmeta/releases/download/v0.1.0-alpha.2/gitmeta-linux-amd64
endif
ifeq ($(UNAME_S),Darwin)
GITMETA := https://github.com/talos-systems/gitmeta/releases/download/v0.1.0-alpha.2/gitmeta-darwin-amd64
endif

BINDIR ?= ./bin
CONFORM_VERSION ?= 57c9dbd

SHA ?= $(shell $(BINDIR)/gitmeta git sha)
TAG ?= $(shell $(BINDIR)/gitmeta image tag)

COMMON_ARGS = --progress=plain
COMMON_ARGS += --frontend=dockerfile.v0
COMMON_ARGS += --allow security.insecure
COMMON_ARGS += --local context=.
COMMON_ARGS += --local dockerfile=.
COMMON_ARGS += --opt build-arg:TOOLS=$(TOOLS)
COMMON_ARGS += --opt build-arg:SHA=$(SHA)
COMMON_ARGS += --opt build-arg:TAG=$(TAG)

DOCKER_ARGS ?=
# to allow tests to run containerd
DOCKER_TEST_ARGS = --security-opt seccomp:unconfined --privileged -v /var/lib/containerd/ -v /tmp/

TESTPKGS ?= ./...

all: ci drone

.PHONY: drone
drone: rootfs initramfs kernel binaries installer talos

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
		$(BUILDKIT_CACHE) \
		$(BUILDKIT_IMAGE) \
		--addr $(BUILDKIT_HOST) \
		--allow-insecure-entitlement security.insecure
	@echo "Wait for buildkitd to become available"
	@sleep 5
endif
endif

.PHONY: binaries
binaries: osctl-linux osctl-darwin

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
rootfs: buildkitd
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

.PHONY: generate
generate: buildkitd
	$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
    --output type=local,dest=./ \
		--opt target=$@ \
		$(COMMON_ARGS)

.PHONY: talos-gce
talos-gce:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out --privileged $(DOCKER_ARGS) autonomy/installer:$(TAG) install -n disk -r -p googlecloud -u none
	@tar -C $(PWD)/build -czf $(PWD)/build/$@.tar.gz disk.raw
	@rm -rf $(PWD)/build/disk.raw

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

.PHONY: basic-integration
basic-integration:
	@KUBERNETES_VERSION=v1.15.0 TAG=$(TAG) ./hack/test/$@.sh

.PHONY: e2e
e2e-integration:
    ## TODO(rsmitty): Bump this k8s version back up once the bug is fixed where kubectl can't scale crds
	@KUBERNETES_VERSION=v1.14.4 TAG=latest ./hack/test/$@.sh

.PHONY: test
test: buildkitd
	@mkdir -p build
	@$(BINDIR)/buildctl --addr $(BUILDKIT_HOST) \
		build \
		--opt target=$@ \
		--output type=local,dest=./ \
		--opt build-arg:TESTPKGS=$(TESTPKGS) \
		$(COMMON_ARGS)

.PHONY: lint
lint: buildkitd
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
push: gitmeta
	@docker tag autonomy/installer:$(TAG) autonomy/installer:latest
	@docker push autonomy/installer:$(TAG)
	@docker push autonomy/installer:latest
	@docker tag autonomy/talos:$(TAG) autonomy/talos:latest
	@docker push autonomy/talos:$(TAG)
	@docker push autonomy/talos:latest

.PHONY: clean
clean:
	@-rm -rf build images vendor

.PHONY: talos-azure
talos-azure:
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out \
		--privileged $(DOCKER_ARGS) \
		autonomy/installer:$(TAG) \
		install \
		-n disk \
		-r \
		-p azure \
		-u none \
		-e rootdelay=300
	@docker run --rm -v $(PWD)/build:/out $(DOCKER_ARGS) \
		--entrypoint qemu-img \
		autonomy/installer:$(TAG) \
		convert \
		-f raw \
		-o subformat=fixed,force_size \
		-O vpc /out/disk.raw /out/disk.vhd
	@tar -C $(PWD)/build -czf $(PWD)/build/$@.tar.gz disk.vhd
	@rm -rf $(PWD)/build/disk.raw $(PWD)/build/disk.vhd
