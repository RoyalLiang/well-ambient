FROM node:25-alpine AS web-build
WORKDIR /src/web
RUN npm install pnpm@10.6.5 --global
COPY web/package.json web/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM golang:1.26.1 AS server-build
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETOS
ARG TARGETARCH
ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildTime=${BUILD_TIME}" \
    -o /out/well-ambient ./cmd/server && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w" -o /out/open-access-admin ./cmd/open-access-admin && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath \
    -ldflags="-s -w" -o /out/open-mcp-stdio ./cmd/open-mcp-stdio

FROM registry.cn-hangzhou.aliyuncs.com/rookiehouse/debian:bookworm-slim AS server

COPY --from=server-build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
RUN sed -i 's|http://deb.debian.org/debian|https://mirrors.tuna.tsinghua.edu.cn/debian|g' /etc/apt/sources.list.d/debian.sources && \
    apt-get -o Acquire::Retries=3 -o APT::Update::Error-Mode=any update && \
    apt-get install -y --no-install-recommends python3 python3-venv ca-certificates && rm -rf /var/lib/apt/lists/*
COPY capabilities/plugins/headroom/ /opt/well-ambient/plugins/headroom/
ARG HEADROOM_PYPI_INDEX=https://pypi.tuna.tsinghua.edu.cn/simple
ENV WELL_AMBIENT_HEADROOM_PYTHON=/opt/well-ambient/headroom/bin/python \
    WELL_AMBIENT_HEADROOM_WORKER=/opt/well-ambient/plugins/headroom/worker.py \
    TIKTOKEN_CACHE_DIR=/opt/well-ambient/plugins/headroom/token-cache \
    PYTHONDONTWRITEBYTECODE=1
RUN python3 -m venv /opt/well-ambient/headroom && \
    /opt/well-ambient/headroom/bin/pip --isolated download --no-cache-dir --only-binary=:all: --no-deps --index-url ${HEADROOM_PYPI_INDEX} --dest /tmp/headroom-wheel headroom-ai==0.41.0 && \
    /opt/well-ambient/headroom/bin/python -I /opt/well-ambient/plugins/headroom/verify_wheel.py /tmp/headroom-wheel && \
    /opt/well-ambient/headroom/bin/pip --isolated install --no-cache-dir --index-url ${HEADROOM_PYPI_INDEX} --find-links /tmp/headroom-wheel -r /opt/well-ambient/plugins/headroom/requirements.lock && \
    rm -rf /tmp/headroom-wheel && \
    /opt/well-ambient/headroom/bin/python -I -c "import tiktoken; tiktoken.get_encoding('o200k_base'); tiktoken.get_encoding('cl100k_base')" && \
    /opt/well-ambient/headroom/bin/python -I /opt/well-ambient/plugins/headroom/worker.py </dev/null

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.title="Well Ambient server" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}"


COPY --from=server-build /out/well-ambient /usr/local/bin/well-ambient
COPY --from=server-build /out/open-access-admin /usr/local/bin/open-access-admin
COPY --from=server-build /out/open-mcp-stdio /usr/local/bin/open-mcp-stdio
COPY --chown=1000:1000 .agents/skills/jira-analysis/ .agents/skills/jira-analysis/
COPY --chown=1000:1000 .agents/skills/board-decision/ .agents/skills/board-decision/
COPY --chown=1000:1000 .agents/skills/code-review-reader/ .agents/skills/code-review-reader/
COPY --chown=1000:1000 .agents/skills/runtime-knowledge/ .agents/skills/runtime-knowledge/
COPY --chown=1000:1000 .agents/skills/well-ambient/ .agents/skills/well-ambient/
COPY --chown=1000:1000 capabilities/open/ capabilities/open/
ENV SSL_CERT_FILE=/etc/ssl/certs/ca-certificates.crt

USER 1000:1000
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/well-ambient"]
CMD ["--config", "/etc/well-ambient/config.yaml", "--skip-migrate"]

FROM uhub.service.ucloud.cn/westwell_devops/nginx:1.31.0 AS web
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.title="Well Ambient web" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${COMMIT}" \
      org.opencontainers.image.created="${BUILD_TIME}"
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=web-build /src/web/dist/ /usr/share/nginx/html/
EXPOSE 8080
