SHA := $(shell gitmeta git sha)
TAG := $(shell gitmeta image tag)
BUILT := $(shell gitmeta built)
PUSH := $(shell gitmeta pushable)

KERNEL_IMAGE ?= autonomy/kernel:65ec2e6
TOOLCHAIN_IMAGE ?= autonomy/toolchain:397b293
GOLANG_VERSION ?= 1.11.4

COMMON_ARGS := --progress=plain
COMMON_ARGS += --frontend=dockerfile.v0
COMMON_ARGS += --local context=.
COMMON_ARGS += --local dockerfile=.
COMMON_ARGS += --frontend-opt build-arg:KERNEL_IMAGE=$(KERNEL_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:TOOLCHAIN_IMAGE=$(TOOLCHAIN_IMAGE)
COMMON_ARGS += --frontend-opt build-arg:GOLANG_VERSION=$(GOLANG_VERSION)
COMMON_ARGS += --frontend-opt build-arg:SHA=$(SHA)
COMMON_ARGS += --frontend-opt build-arg:TAG=$(TAG)

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
	@docker pull k8s.gcr.io/$@:v1.13.3
	@docker save k8s.gcr.io/$@:v1.13.3 -o ./images/$@.tar

etcd:
	@docker pull k8s.gcr.io/$@:3.2.24
	@docker save k8s.gcr.io/$@:3.2.24 -o ./images/$@.tar

coredns:
	@docker pull k8s.gcr.io/$@:1.2.6
	@docker save k8s.gcr.io/$@:1.2.6 -o ./images/$@.tar

pause:
	@docker pull k8s.gcr.io/$@:3.1
	@docker save k8s.gcr.io/$@:3.1 -o ./images/$@.tar

clean:
	-go clean -modcache
	-rm -rf build vendor
