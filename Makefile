VERSION ?= $(shell git rev-parse --short=12 HEAD)
SERVER_IMAGE ?= well-ambient-server
WEB_IMAGE ?= well-ambient-web
PLATFORM ?= linux/amd64
IMAGE_BUNDLE ?= deploy/bundles/well-ambient-images-$(VERSION).tar
COMPOSE_BUNDLE ?= deploy/bundles/well-ambient-compose-$(VERSION).tar.gz

.PHONY: verify images image-bundle compose-bundle deploy rollback

verify:
	GOCACHE=/tmp/well-ambient-gocache go test ./...
	GOCACHE=/tmp/well-ambient-gocache go vet ./...
	pnpm -C web check
	pnpm -C web build

images:
	docker build --platform $(PLATFORM) --target server \
		--build-arg VERSION=$(VERSION) --build-arg COMMIT=$(VERSION) \
		--build-arg BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
		-t $(SERVER_IMAGE):$(VERSION) .
	docker build --platform $(PLATFORM) --target web \
		-t $(WEB_IMAGE):$(VERSION) .

image-bundle: images
	mkdir -p $$(dirname $(IMAGE_BUNDLE))
	docker save -o $(IMAGE_BUNDLE) \
		$(SERVER_IMAGE):$(VERSION) $(WEB_IMAGE):$(VERSION)

compose-bundle:
	mkdir -p $$(dirname $(COMPOSE_BUNDLE))
	tar -czf $(COMPOSE_BUNDLE) \
		compose.yaml \
		deploy/.env.production.example \
		deploy/config.production.example.yaml \
		deploy/deploy.sh \
		deploy/rollback.sh

deploy:
	./deploy/deploy.sh $(VERSION)

rollback:
	./deploy/rollback.sh
