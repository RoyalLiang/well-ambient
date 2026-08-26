VERSION ?= $(shell git rev-parse --short=12 HEAD)

.PHONY: verify images deploy rollback

verify:
	GOCACHE=/tmp/well-ambient-gocache go test ./...
	GOCACHE=/tmp/well-ambient-gocache go vet ./...
	pnpm -C web check
	pnpm -C web build

images:
	WELL_AMBIENT_VERSION=$(VERSION) WELL_AMBIENT_COMMIT=$(VERSION) WELL_AMBIENT_BUILD_TIME=$$(date -u +%Y-%m-%dT%H:%M:%SZ) \
		docker compose --env-file deploy/.env.production build server web

deploy:
	./deploy/deploy.sh $(VERSION)

rollback:
	./deploy/rollback.sh
