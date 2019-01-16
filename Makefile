SHA := $(shell gitmeta git sha)
TAG := $(shell gitmeta image tag)
BUILT := $(shell gitmeta built)

COMMON_APP_ARGS := -f ./Dockerfile --build-arg TOOLCHAIN_VERSION=690a03a --build-arg KERNEL_VERSION=e18620a --build-arg GOLANG_VERSION=1.11.4 --build-arg SHA=$(SHA) --build-arg TAG=$(TAG) .

export DOCKER_BUILDKIT := 1

all: enforce rootfs initramfs osctl udevd test installer docs

enforce:
	@docker run --rm -it -v $(PWD):/src -w /src autonomy/conform:latest

osd:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker save autonomy/$@:$(TAG) -o ./images/$@.tar

osctl:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(TAG) cp /osctl-linux-amd64 /build
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(TAG) cp /osctl-darwin-amd64 /build

trustd:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker save autonomy/$@:$(TAG) -o ./images/$@.tar

proxyd:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker save autonomy/$@:$(TAG) -o ./images/$@.tar

blockd:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker save autonomy/$@:$(TAG) -o ./images/$@.tar

udevd:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS) \

test:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)

hyperkube:
	@docker pull k8s.gcr.io/$@:v1.13.1
	@docker save k8s.gcr.io/$@:v1.13.1 -o ./images/$@.tar

etcd:
	@docker pull k8s.gcr.io/$@:3.2.24
	@docker save k8s.gcr.io/$@:3.2.24 -o ./images/$@.tar

coredns:
	@docker pull k8s.gcr.io/$@:1.2.6
	@docker save k8s.gcr.io/$@:1.2.6 -o ./images/$@.tar

pause:
	@docker pull k8s.gcr.io/$@:3.1
	@docker save k8s.gcr.io/$@:3.1 -o ./images/$@.tar

rootfs: hyperkube etcd coredns pause osd trustd proxyd blockd
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(TAG) cp /rootfs.tar.gz /build

initramfs:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(TAG) cp /initramfs.xz /build

.PHONY: docs
docs:
	@docker build \
		-t autonomy/$@:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@rm -rf ./docs
	@docker run --rm -it -v $(PWD):/out autonomy/$@:$(TAG) cp -R /docs /out

installer:
	@docker build \
		-t autonomy/talos:$(TAG) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/talos:$(TAG) cp /generated/boot/vmlinuz /build
	@docker run --rm -it -v /dev:/dev -v $(PWD)/build:/out --privileged autonomy/talos:$(TAG) image -l

deps:
	@GO111MODULES=on CGO_ENABLED=0 go get -u github.com/autonomy/gitmeta
	@GO111MODULES=on CGO_ENABLED=0 go get -u github.com/autonomy/conform

clean:
	go clean -modcache
	rm -rf build vendor
