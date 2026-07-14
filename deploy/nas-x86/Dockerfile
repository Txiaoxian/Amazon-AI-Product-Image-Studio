# syntax=docker/dockerfile:1.7

ARG NODE_IMAGE=docker.m.daocloud.io/library/node:24-alpine
ARG GO_IMAGE=docker.m.daocloud.io/library/golang:1.25-alpine
ARG MINIO_IMAGE=docker.m.daocloud.io/minio/minio:RELEASE.2025-04-22T22-12-26Z
ARG MINIO_MC_IMAGE=docker.m.daocloud.io/minio/mc:RELEASE.2025-04-16T18-13-26Z
ARG RUNTIME_IMAGE=docker.m.daocloud.io/library/ubuntu:24.04

FROM --platform=$BUILDPLATFORM ${NODE_IMAGE} AS frontend-build

WORKDIR /src/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY frontend/ ./
RUN npm run build

FROM --platform=$BUILDPLATFORM ${GO_IMAGE} AS backend-build

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY backend/cmd ./cmd
COPY backend/internal ./internal

ARG TARGETOS=linux
ARG TARGETARCH
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/backend-api ./cmd/api \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/backend-worker ./cmd/worker \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/provision-tenant ./cmd/provision-tenant \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags='-s -w' -o /out/provider-key-rotation ./cmd/provider-key-rotation

FROM ${MINIO_IMAGE} AS minio-binary
FROM ${MINIO_MC_IMAGE} AS minio-client-binary

FROM ${RUNTIME_IMAGE} AS runtime

ARG DEBIAN_FRONTEND=noninteractive
RUN set -eux; \
    printf '#!/bin/sh\nexit 101\n' > /usr/sbin/policy-rc.d; \
    chmod +x /usr/sbin/policy-rc.d; \
    apt-get update; \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      mysql-client \
      mysql-server \
      nginx \
      openssl \
      redis-server \
      redis-tools \
      supervisor \
      tzdata; \
    rm -f /usr/sbin/policy-rc.d; \
    rm -rf /var/lib/apt/lists/* /var/lib/mysql/* /var/lib/redis/*; \
    groupadd --gid 10001 studio; \
    useradd --uid 10001 --gid 10001 --no-create-home --shell /usr/sbin/nologin studio; \
    mkdir -p /opt/studio /usr/share/nginx/html

COPY --from=frontend-build /src/frontend/dist/ /usr/share/nginx/html/
COPY --from=backend-build /out/backend-api /usr/local/bin/backend-api
COPY --from=backend-build /out/backend-worker /usr/local/bin/backend-worker
COPY --from=backend-build /out/provision-tenant /usr/local/bin/provision-tenant
COPY --from=backend-build /out/provider-key-rotation /usr/local/bin/provider-key-rotation
COPY --from=minio-binary /usr/bin/minio /usr/local/bin/minio
COPY --from=minio-client-binary /usr/bin/mc /usr/local/bin/mc

COPY deploy/nas-x86/config/nginx.conf /etc/nginx/sites-available/default
COPY deploy/nas-x86/config/mysql.cnf /etc/mysql/mysql.conf.d/zz-studio.cnf
COPY deploy/nas-x86/config/supervisord.conf /etc/supervisor/conf.d/studio.conf
COPY deploy/nas-x86/scripts/ /usr/local/bin/

RUN set -eux; \
    chmod 0755 \
      /usr/local/bin/backend-api \
      /usr/local/bin/backend-worker \
      /usr/local/bin/provision-tenant \
      /usr/local/bin/provider-key-rotation \
      /usr/local/bin/minio \
      /usr/local/bin/mc \
      /usr/local/bin/entrypoint.sh \
      /usr/local/bin/healthcheck.sh \
      /usr/local/bin/start-backend-api.sh \
      /usr/local/bin/start-backend-worker.sh \
      /usr/local/bin/start-minio.sh; \
    rm -f /etc/nginx/sites-enabled/default; \
    ln -s /etc/nginx/sites-available/default /etc/nginx/sites-enabled/default; \
    rm -f /etc/supervisor/conf.d/supervisord.conf

ENV TZ=Asia/Shanghai \
    BACKEND_HTTP_HOST=127.0.0.1 \
    BACKEND_HTTP_PORT=8081 \
    MYSQL_HOST=127.0.0.1 \
    MYSQL_PORT=3306 \
    REDIS_ADDR=127.0.0.1:6379 \
    MINIO_ENDPOINT=http://127.0.0.1:9000 \
    WORKER_HEALTHCHECK_FILE=/run/studio/worker-ready \
    TMPDIR=/data/tmp

EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=15s --timeout=10s --start-period=180s --retries=8 \
  CMD ["/usr/local/bin/healthcheck.sh"]

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
