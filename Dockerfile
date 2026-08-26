# syntax=docker/dockerfile:1.7

FROM node:24-bookworm-slim AS web-build
WORKDIR /src/web
RUN corepack enable && corepack prepare pnpm@10.6.5 --activate
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.24.13-bookworm AS server-build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETOS=linux
ARG TARGETARCH=amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /out/well-ambient ./cmd/server

FROM debian:bookworm-slim AS server
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl tzdata \
    && rm -rf /var/lib/apt/lists/* \
    && groupadd --gid 10001 ambient \
    && useradd --uid 10001 --gid ambient --no-create-home --shell /usr/sbin/nologin ambient \
    && mkdir -p /var/lib/well-ambient/attachments /etc/well-ambient \
    && chown -R ambient:ambient /var/lib/well-ambient /etc/well-ambient
COPY --from=server-build /out/well-ambient /usr/local/bin/well-ambient
USER ambient
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/well-ambient"]
CMD ["--config", "/etc/well-ambient/config.yaml", "--skip-migrate"]

FROM nginx:1.28.3-alpine AS web
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=web-build /src/web/dist/ /usr/share/nginx/html/
EXPOSE 8080
