# Local Development Environment

## Source of truth

Local development for this project uses the shared machine-level environment documented in:

```text
/Users/wohenhaoqi/.codex/agent-instructions/10-local-dev-environment.md
```

That global document is the source of truth for local service credentials. Do not copy real local passwords or secrets into this repository.

## Development policy

- Use the existing global local services for feature development and validation.
- Do not start project-specific MySQL, Redis, or MinIO containers for normal backend, frontend, auth, asset, Provider, task, or SSE work.
- Do not create project-specific Docker volumes for routine development validation.
- `deploy/docker-compose.yml` remains the deployment topology and may be used for deployment-specific verification only.
- If a deployment-specific test starts the project Compose stack, clean it up after verification unless the user explicitly asks to keep it running:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

## Shared services

### Go

- Installed through Homebrew.
- Global document currently records `go version go1.26.3 darwin/arm64`.
- Verify with:

```bash
go version
go env GOPATH
```

### Shared Docker Compose

- Compose file:

```text
/Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml
```

- Data root:

```text
/Volumes/wohenhaoqi/data/ApplicationsData/dev-env
```

- Check status:

```bash
docker compose -f /Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml ps
```

- Start shared services if stopped:

```bash
docker compose -f /Volumes/wohenhaoqi/data/ApplicationsData/dev-env/compose/docker-compose.yml start
```

### MySQL 8

- Container: `dev-mysql8`.
- Image: `mysql:8.0`.
- Current version in global document: `8.0.46`.
- Host endpoint: `127.0.0.1:3306`.
- User for local validation: `root`.
- Password: read from the global local development document; do not duplicate it here.
- Data directory:

```text
/Volumes/wohenhaoqi/data/ApplicationsData/dev-env/mysql8/data
```

- Project database name:

```text
amazon_ai_image_studio
```

- Create the project database if needed:

```bash
MYSQL_PWD='<read from global local dev document>' docker exec dev-mysql8 \
  mysql -uroot -e "CREATE DATABASE IF NOT EXISTS amazon_ai_image_studio CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci;"
```

- Verify MySQL:

```bash
MYSQL_PWD='<read from global local dev document>' docker exec dev-mysql8 \
  mysql -uroot -e "SHOW VARIABLES WHERE Variable_name IN ('lower_case_table_names','character_set_server','collation_server'); SELECT @@sql_mode;"
```

### Redis

- Container: `dev-redis`.
- Image: `redis:7`.
- Current version in global document: `7.4.9`.
- Host endpoint: `127.0.0.1:6379`.
- Password: empty for local development.
- Data directory:

```text
/Volumes/wohenhaoqi/data/ApplicationsData/dev-env/redis/data
```

- Verify Redis:

```bash
docker exec dev-redis redis-cli ping
```

### MinIO

- Container: `dev-minio`.
- Image:

```text
docker.m.daocloud.io/minio/minio:RELEASE.2025-04-22T22-12-26Z
```

- S3 API endpoint: `http://127.0.0.1:9000`.
- Console endpoint: `http://127.0.0.1:9001`.
- Access key / root user: `minioadmin`.
- Secret key / root password: read from the global local development document; do not duplicate it here.
- Data directory:

```text
/Volumes/wohenhaoqi/data/ApplicationsData/dev-env/minio/data
```

- Project buckets:

```text
product-originals
product-generated
product-thumbnails
```

- Verify MinIO:

```bash
curl -fsS http://127.0.0.1:9000/minio/health/live
nc -vz 127.0.0.1 9000
nc -vz 127.0.0.1 9001
```

## Backend local environment

For local backend runs and integration checks, use environment variables that point to the shared services:

```bash
export MYSQL_HOST=127.0.0.1
export MYSQL_PORT=3306
export MYSQL_DATABASE=amazon_ai_image_studio
export MYSQL_USER=root
export MYSQL_PASSWORD='<read from global local dev document>'
export REDIS_ADDR=127.0.0.1:6379
export REDIS_PASSWORD=
export MINIO_ENDPOINT=http://127.0.0.1:9000
export MINIO_ACCESS_KEY=minioadmin
export MINIO_SECRET_KEY='<read from global local dev document>'
export MINIO_REGION=us-east-1
```

Do not commit a local `.env` file that contains these values.

## Verification guidance

Normal feature validation should prefer:

```bash
cd backend
go test ./...
go test -race ./...
go vet ./...
```

Use the shared service health checks above when a feature needs MySQL, Redis, or MinIO integration.

Deployment verification may still use:

```bash
docker compose -f deploy/docker-compose.yml config
docker compose -f deploy/docker-compose.yml build backend-api backend-worker frontend
```

Only run `docker compose -f deploy/docker-compose.yml up -d` for deployment-specific validation, and clean it up after the check unless instructed otherwise.

## P9 deployment validation note

`P9-DEPLOY-RELEASE-VALIDATION` is the deployment-specific exception to the shared-service rule. It may start the project Compose stack to validate release topology, including the one-shot `minio-bootstrap` service that creates or verifies the required MinIO buckets idempotently.

After validation, clean all project-specific resources:

```bash
docker compose -f deploy/docker-compose.yml down -v --remove-orphans
```

Do not copy shared local service credentials into `.env`, `.env.example`, Compose files, documentation, logs, or final handoff notes.
