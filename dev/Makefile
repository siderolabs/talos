SHELL := /bin/bash

ORG = talos.local
K_ORG = talos.k8s.local
USER = dev
CA_LENGTH = $$(( 24 * 365 * 1 ))
K_CA_LENGTH = $$(( 24 * 365 * 1 ))
USER_LENGTH = $$(( 24 * 30 * 5 ))

NODE = talos-os-1

GIT_SHORT_SHA = $(shell git rev-parse --short=7 HEAD)
GIT_DIRTY = $(shell [[ -n `git status -s` ]] && echo -dirty)
TAG = $(GIT_SHORT_SHA)$(GIT_DIRTY)
# TAG = $(shell gitmeta image tag)  # TODO: this tool is slow

OS_IMAGE_NAME ?= autonomy/talos-os

UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
OSCTL ?= ../build/osctl-linux-amd64
endif
ifeq ($(UNAME_S),Darwin)
OSCTL ?= ../build/osctl-darwin-amd64
endif


DOCKER_COMPOSE ?= IMAGE=$(OS_IMAGE_NAME) TAG=$(TAG) NODE=$(NODE) docker-compose
.PHONY: os-up os-down os-clean
os-up :
	touch os1.kmsg
	mkdir -p os1.logs/pods/ && chmod -R 777 os1.logs  # easier to manage pod logs
	$(DOCKER_COMPOSE) up -d
	tail -f os1.kmsg
os-down :
	$(DOCKER_COMPOSE) down
os-clean :
	$(DOCKER_COMPOSE) down -v
	rm -rf os1.kmsg os1.logs || true


.PHONY: config config-clean
config : config/$(NODE).yaml $(HOME)/.talos/config
config-clean:
	rm config/* || true  # does not delete dotfiles


pki/ :
	@mkdir -p pki/

# OS CA PKI
pki/$(ORG).key : pki/
	# gen OS CA
	@$(OSCTL) gen ca \
		--hours "$(CA_LENGTH)" \
		--organization "pki/$(ORG)"
pki/$(ORG).crt : pki/$(ORG).key
pki/$(ORG).sha256 : pki/$(ORG).key

# user PKI
pki/$(USER).key : pki/
	# gen user key
	@$(OSCTL) gen key \
		--name "pki/$(USER)"
pki/$(USER).csr : pki/$(USER).key
	# gen user CSR
	@$(OSCTL) gen csr \
		--ip "127.0.0.1" \
		--key "pki/$(USER).key"
pki/$(USER).crt : pki/$(USER).csr pki/$(ORG).crt
	# gen user crt
	@$(OSCTL) gen crt \
		--hours "$(USER_LENGTH)" \
		--ca "pki/$(ORG)" \
		--csr "pki/$(USER).csr" \
		--name "pki/$(USER)"

# k8s CA PKI
pki/$(K_ORG).key : pki/
	# gen k8s CA
	@$(OSCTL) gen ca \
		--rsa \
		--hours "$(K_CA_LENGTH)" \
		--organization "pki/$(K_ORG)"
pki/$(K_ORG).crt : pki/$(K_ORG).key
pki/$(K_ORG).sha256 : pki/$(K_ORG).key

# PKI cleanup
.PHONY: pki-clean-all pki-clean-os pki-clean-user pki-clean-k8s
pki-clean-all:
	rm -rf pki || true
pki-clean-os:
	rm -f pki/$(ORG)* || true
pki-clean-user:
	rm -f pki/$(USER)* || true
pki-clean-k8s:
	rm -f pki/$(K_ORG)* || true


config/$(NODE).yaml : pki/$(ORG).crt pki/$(ORG).key pki/$(K_ORG).crt pki/$(K_ORG).key
	# inject base os config
	@cp config/.base.node.yaml "config/$(NODE).yaml"
	@chmod 600 "config/$(NODE).yaml"
	@$(OSCTL) inject os \
		--crt "pki/$(ORG).crt" \
		--key "pki/$(ORG).key" \
		"config/$(NODE).yaml"
	@$(OSCTL) inject kubernetes \
		--crt "pki/$(K_ORG).crt" \
		--key "pki/$(K_ORG).key" \
		"config/$(NODE).yaml"

$(HOME)/.talos/config : pki/$(ORG).crt pki/$(USER).crt pki/$(USER).key
	# configure dev osctl
	@touch $${HOME}/.talos/config
	@$(OSCTL) config add "talos-local" \
		--ca "pki/$(ORG).crt" \
		--crt "pki/$(USER).crt" \
		--key "pki/$(USER).key"
	@$(OSCTL) config context "talos-local"
	@$(OSCTL) config target "127.0.0.1"


.PHONY: clean
clean : config-clean os-clean pki-clean-all
