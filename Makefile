SHA := $(shell gitmeta git sha)
TAG := $(shell gitmeta image tag)
BUILT := $(shell gitmeta built)

COMMON_APP_ARGS := -f ./Dockerfile --build-arg TOOLCHAIN_VERSION=690a03a --build-arg KERNEL_VERSION=e18620a --build-arg GOLANG_VERSION=1.11.4 --build-arg SHA=$(SHA) --build-arg TAG=$(TAG) .

export DOCKER_BUILDKIT := 1

all: enforce rootfs initramfs osd osctl trustd proxyd blockd udevd test installer docs

enforce:
	@docker run --rm -it -v $(PWD):/src -w /src autonomy/conform:latest

osd:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)

osctl:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(SHA) cp /osctl-linux-amd64 /build
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(SHA) cp /osctl-darwin-amd64 /build

trustd:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)

proxyd:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)

blockd:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)

udevd:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS) \

test:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)

rootfs:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(SHA) cp /rootfs.tar.gz /build

initramfs:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/$@:$(SHA) cp /initramfs.xz /build

.PHONY: docs
docs:
	@docker build \
		-t autonomy/$@:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@rm -rf ./docs
	@docker run --rm -it -v $(PWD):/out autonomy/$@:$(SHA) cp -R /docs /out

.PHONY: installer
installer:
	@docker build \
		-t autonomy/talos:$(SHA) \
		--target=$@ \
		$(COMMON_APP_ARGS)
	@docker run --rm -it -v $(PWD)/build:/build autonomy/talos:$(SHA) cp /generated/boot/vmlinuz /build

deps:
	@GO111MODULES=on CGO_ENABLED=0 go get -u github.com/autonomy/gitmeta
	@GO111MODULES=on CGO_ENABLED=0 go get -u github.com/autonomy/conform

clean:
	go clean -modcache
	rm -rf build vendor
