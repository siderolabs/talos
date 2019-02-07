SHA := $(shell gitmeta git sha)
TAG := $(shell gitmeta image tag)
BUILT := $(shell gitmeta built)
PUSH := $(shell gitmeta pushable)

KERNEL_IMAGE ?= autonomy/kernel:65ec2e6
TOOLCHAIN_IMAGE ?= autonomy/toolchain:397b293
GOLANG_VERSION ?= 1.11.4
HYPERKUBE_IMAGE ?= k8s.gcr.io/hyperkube:v1.13.3
ETCD_IMAGE ?= k8s.gcr.io/etcd:3.2.24
COREDNS_IMAGE ?= k8s.gcr.io/coredns:1.2.6
PAUSE_IMAGE ?= k8s.gcr.io/pause:3.1

COMMON_DOCKER_ARGS += --build-arg KERNEL_IMAGE=$(KERNEL_IMAGE)
COMMON_DOCKER_ARGS += --build-arg TOOLCHAIN_IMAGE=$(TOOLCHAIN_IMAGE)
COMMON_DOCKER_ARGS += --build-arg HYPERKUBE_IMAGE=$(HYPERKUBE_IMAGE)
COMMON_DOCKER_ARGS += --build-arg ETCD_IMAGE=$(ETCD_IMAGE)
COMMON_DOCKER_ARGS += --build-arg COREDNS_IMAGE=$(COREDNS_IMAGE)
COMMON_DOCKER_ARGS += --build-arg PAUSE_IMAGE=$(PAUSE_IMAGE)
COMMON_DOCKER_ARGS += --build-arg GOLANG_VERSION=$(GOLANG_VERSION)
COMMON_DOCKER_ARGS += --progress=plain

COMMON_ARGS := --progress=plain
COMMON_ARGS += --frontend=dockerfile.v0
COMMON_ARGS += --local context=.
COMMON_ARGS += --local dockerfile=.
COMMON_ARGS += --frontend-opt build-arg:KERNEL_IMAGE=$(KERNEL_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:TOOLCHAIN_IMAGE=$(TOOLCHAIN_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:GOLANG_VERSION=$(GOLANG_VERSION)
COMMON_ARGS += --frontend-opt build-arg:SHA=$(SHA)
COMMON_ARGS += --frontend-opt build-arg:TAG=$(TAG)
COMMON_ARGS += --frontend-opt build-arg:HYPERKUBE_IMAGE=$(HYPERKUBE_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:ETCD_IMAGE=$(ETCD_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:COREDNS_IMAGE=$(COREDNS_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:PAUSE_IMAGE=$(PAUSE_IMAGE)

all: kernel initramfs rootfs osctl-linux-amd64 osctl-darwin-amd64 test lint docs installer

base:
	@buildctl build \
		--exporter=docker \
		--exporter-opt output=build/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar

kernel:
	@buildctl build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

initramfs:
	@buildctl build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

rootfs: hyperkube etcd coredns pause osd trustd proxyd blockd
	@buildctl build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)
	@./hack/scripts/warm.sh

installer:
	@buildctl build \
		--exporter=docker \
		--exporter-opt output=build/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--exporter-opt push=$(PUSH) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)
	@docker load < build/$@.tar
	@docker run --rm -v /dev:/dev -v $(PWD)/build:/out --privileged autonomy/$@:$(TAG) image -l

.PHONY: docs
docs:
	@rm -rf ./docs
	@buildctl build \
		--exporter=local \
		--exporter-opt output=. \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

test:
	@buildctl build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

lint:
	@buildctl build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

osctl-linux-amd64:
	@buildctl build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

osctl-darwin-amd64:
	@buildctl build \
		--exporter=local \
		--exporter-opt output=build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

udevd:
	@buildctl build \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

osd:
	@buildctl build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

trustd:
	@buildctl build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

proxyd:
	@buildctl build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

blockd:
	@buildctl build \
		--exporter=docker \
		--exporter-opt output=images/$@.tar \
		--exporter-opt name=docker.io/autonomy/$@:$(TAG) \
		--frontend-opt target=$@ \
		$(COMMON_ARGS)

hyperkube:
	@docker build --squash --target=$@ $(COMMON_DOCKER_ARGS) -t $(HYPERKUBE_IMAGE) .
	@docker save $(HYPERKUBE_IMAGE) -o ./images/$@.tar

etcd:
	@docker build --squash --target=$@ $(COMMON_DOCKER_ARGS) -t $(ETCD_IMAGE) .
	@docker save $(ETCD_IMAGE) -o ./images/$@.tar

coredns:
	@docker build --squash --target=$@ $(COMMON_DOCKER_ARGS) -t $(COREDNS_IMAGE) .
	@docker save $(COREDNS_IMAGE) -o ./images/$@.tar

pause:
	@docker build --squash --target=$@ $(COMMON_DOCKER_ARGS) -t $(PAUSE_IMAGE) .
	@docker save $(PAUSE_IMAGE) -o ./images/$@.tar

clean:
	-go clean -modcache
	-rm -rf build vendor
