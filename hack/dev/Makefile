SHELL := /bin/bash

OS_IMAGE_NAME ?= autonomy/talos
TAG = $(shell gitmeta image tag)
DOCKER_COMPOSE ?= IMAGE=$(OS_IMAGE_NAME) TAG=$(TAG) docker-compose
IP_ADDR ?= 10.5.0.6

export TALOSCONFIG = $(PWD)/talosconfig

all: up

.PHONY: up
up: talosconfig
	@$(DOCKER_COMPOSE) up -d $(SERVICES)

.PHONY: down
down:
	@$(DOCKER_COMPOSE) down -v $(SERVICES)

talosconfig:
	@mkdir -p pki
	@./gen.sh $(IP_ADDR)

kubeconfig:
	@./osctl.sh kubeconfig > kubeconfig

enter: kubeconfig
	@./enter.sh

.PHONY: manifests
manifests: kubeconfig
	@./kubectl.sh apply -f /manifests/psp.yaml
	@./kubectl.sh apply -f /manifests/cni.yaml
	@./kubectl.sh apply -f /manifests/coredns.yaml

integration: up
	@./integration.sh

.PHONY: clean
clean: down
	-@rm talosconfig
	-@rm kubeconfig
	-@rm userdata/master-1.yaml
	-@rm userdata/master-2.yaml
	-@rm userdata/master-3.yaml
	-@rm userdata/worker-1.yaml
	-@rm -rf pki
	-@rm -rf logs
