SHELL := /bin/bash
.SHELLFLAGS := -eu -o pipefail -c

VERSION ?=
COMMIT ?=
BUILD_TIME ?=
PLATFORM ?= linux/amd64
GOPROXY ?= https://goproxy.cn,direct
RELEASE_DIR := deploy/generated
RELEASE_ENV := $(RELEASE_DIR)/release.env
IMAGE_BUNDLE ?=
COMPOSE_BUNDLE ?=

PRODUCTION_SERVER_IMAGE := $(shell sed -n 's/^WELL_AMBIENT_SERVER_IMAGE=//p' deploy/.env.production 2>/dev/null | tail -n 1 | tr -d '[:space:]')
PRODUCTION_WEB_IMAGE := $(shell sed -n 's/^WELL_AMBIENT_WEB_IMAGE=//p' deploy/.env.production 2>/dev/null | tail -n 1 | tr -d '[:space:]')
SERVER_IMAGE ?= $(if $(PRODUCTION_SERVER_IMAGE),$(PRODUCTION_SERVER_IMAGE),well-ambient-server)
WEB_IMAGE ?= $(if $(PRODUCTION_WEB_IMAGE),$(PRODUCTION_WEB_IMAGE),well-ambient-web)

.PHONY: dev-setup verify release-metadata release-info images image-bundle compose-bundle release deploy rollback

dev-setup:
	./scripts/dev-setup.sh

verify:
	GOPROXY=$(GOPROXY) GOCACHE=/tmp/well-ambient-gocache go test ./...
	GOPROXY=$(GOPROXY) GOCACHE=/tmp/well-ambient-gocache go vet ./...
	pnpm -C web check
	pnpm -C web build

release-metadata:
	VERSION="$(VERSION)" COMMIT="$(COMMIT)" BUILD_TIME="$(BUILD_TIME)" \
		./scripts/release-metadata.sh --output-dir "$(RELEASE_DIR)"

release-info: release-metadata
	@. "$(RELEASE_ENV)"; \
		printf 'version: %s\nbuild time: %s\nbatch: %s\n' \
		"$$WELL_AMBIENT_VERSION" "$$WELL_AMBIENT_BUILD_TIME" "$$WELL_AMBIENT_RELEASE_BATCH"

images: release-metadata
	. "$(RELEASE_ENV)"; \
		docker build --platform "$(PLATFORM)" --target server \
			--build-arg GOPROXY="$(GOPROXY)" \
			--build-arg VERSION="$$WELL_AMBIENT_VERSION" \
			--build-arg COMMIT="$$WELL_AMBIENT_COMMIT" \
			--build-arg BUILD_TIME="$$WELL_AMBIENT_BUILD_TIME" \
			-t "$(SERVER_IMAGE):$$WELL_AMBIENT_VERSION" .; \
		docker build --platform "$(PLATFORM)" --target web \
			--build-arg VERSION="$$WELL_AMBIENT_VERSION" \
			--build-arg COMMIT="$$WELL_AMBIENT_COMMIT" \
			--build-arg BUILD_TIME="$$WELL_AMBIENT_BUILD_TIME" \
			-t "$(WEB_IMAGE):$$WELL_AMBIENT_VERSION" .

image-bundle: images
	. "$(RELEASE_ENV)"; \
		image_bundle="$(IMAGE_BUNDLE)"; \
		if [[ -z "$$image_bundle" ]]; then image_bundle="deploy/bundles/well-ambient-images-$$WELL_AMBIENT_VERSION.tar.gz"; fi; \
		mkdir -p "$$(dirname "$$image_bundle")"; \
		tmp_bundle="$$image_bundle.tmp"; \
		trap 'rm -f "$$tmp_bundle"' EXIT; \
		docker save "$(SERVER_IMAGE):$$WELL_AMBIENT_VERSION" "$(WEB_IMAGE):$$WELL_AMBIENT_VERSION" | gzip -9 >"$$tmp_bundle"; \
		mv "$$tmp_bundle" "$$image_bundle"; \
		trap - EXIT; \
		printf 'image bundle: %s\n' "$$image_bundle"

compose-bundle: release-metadata
	. "$(RELEASE_ENV)"; \
		compose_bundle="$(COMPOSE_BUNDLE)"; \
		if [[ -z "$$compose_bundle" ]]; then compose_bundle="deploy/bundles/well-ambient-compose-$$WELL_AMBIENT_VERSION.tar.gz"; fi; \
		mkdir -p "$$(dirname "$$compose_bundle")"; \
		tmp_bundle="$$compose_bundle.tmp"; \
		trap 'rm -f "$$tmp_bundle"' EXIT; \
		tar -czf "$$tmp_bundle" \
			compose.yaml \
			deploy/.env.production.example \
			deploy/config.production.example.yaml \
			deploy/deploy.sh \
			deploy/rollback.sh \
			deploy/generated/release.env \
			deploy/generated/release-notes.txt; \
		mv "$$tmp_bundle" "$$compose_bundle"; \
		trap - EXIT; \
		printf 'compose bundle: %s\n' "$$compose_bundle"

release: verify image-bundle compose-bundle
	@. "$(RELEASE_ENV)"; \
		printf 'release %s is ready; batch: %s\n' "$$WELL_AMBIENT_VERSION" "$$WELL_AMBIENT_RELEASE_BATCH"

deploy: images
	WELL_AMBIENT_RELEASE_ENV="$(abspath $(RELEASE_ENV))" ./deploy/deploy.sh

rollback:
	./deploy/rollback.sh
