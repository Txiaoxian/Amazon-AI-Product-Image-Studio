#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd -- "${SCRIPT_DIR}/../.." && pwd)"
VERSION="${1:-$(date '+%Y%m%d-%H%M%S')}"
OUTPUT_ROOT="${2:-${REPO_ROOT}/dist/nas-x86}"
IMAGE_NAME="${IMAGE_NAME:-amazon-ai-product-image-studio-nas}"
IMAGE_REF="${IMAGE_NAME}:${VERSION}"
PACKAGE_NAME="gpt-image-nas-amd64-${VERSION}"
IMAGE_ARCHIVE_NAME="${PACKAGE_NAME}.docker.tar"
DIRECT_IMAGE_ARCHIVE="${OUTPUT_ROOT}/${IMAGE_ARCHIVE_NAME}"
IMAGE_CHECKSUM="${DIRECT_IMAGE_ARCHIVE}.sha256"
IMAGE_README="${OUTPUT_ROOT}/${PACKAGE_NAME}.README.md"

die() {
  printf '构建失败：%s\n' "$*" >&2
  exit 1
}

command -v docker >/dev/null 2>&1 || die '未找到 Docker，请先启动 Docker Desktop'
docker info >/dev/null 2>&1 || die 'Docker daemon 不可用，请先启动 Docker Desktop'
docker buildx version >/dev/null 2>&1 || die '当前 Docker 未安装 buildx'
[[ "${VERSION}" =~ ^[A-Za-z0-9][A-Za-z0-9_.-]*$ ]] || die '版本号只能包含字母、数字、点、下划线和横线'
[[ "${IMAGE_NAME}" =~ ^[A-Za-z0-9._/-]+$ ]] || die 'IMAGE_NAME 格式不合法'

bash "${SCRIPT_DIR}/test-package.sh"

if [[ "${SKIP_TESTS:-0}" != "1" ]]; then
  printf '运行后端测试...\n'
  (cd "${REPO_ROOT}/backend" && go test ./...)
  printf '使用 Node 24 容器运行前端质量检查...\n'
  docker run --rm \
    --user "$(id -u):$(id -g)" \
    --env HOME=/tmp \
    --env npm_config_cache=/tmp/.npm \
    --volume "${REPO_ROOT}/frontend:/workspace" \
    --workdir /workspace \
    "${FRONTEND_TEST_NODE_IMAGE:-docker.m.daocloud.io/library/node:24-alpine}" \
    sh -lc 'npm ci && npm run lint && npm run type-check && npm test -- --no-file-parallelism && npm audit --omit=dev --audit-level=high'
else
  printf '警告：SKIP_TESTS=1，已跳过源码测试。\n' >&2
fi

mkdir -p "${OUTPUT_ROOT}"
rm -f "${DIRECT_IMAGE_ARCHIVE}" "${IMAGE_CHECKSUM}" "${IMAGE_README}"

printf '构建 linux/amd64 单容器镜像 %s...\n' "${IMAGE_REF}"
docker buildx build \
  --platform linux/amd64 \
  --file "${SCRIPT_DIR}/Dockerfile" \
  --tag "${IMAGE_REF}" \
  --label "org.opencontainers.image.title=Amazon AI Product Image Studio NAS" \
  --label "org.opencontainers.image.version=${VERSION}" \
  --label "org.opencontainers.image.source=local-workspace" \
  --output "type=docker,dest=${DIRECT_IMAGE_ARCHIVE}" \
  "${REPO_ROOT}"

cp "${REPO_ROOT}/docs/nas-x86-deployment.md" "${IMAGE_README}"

if [[ "${SKIP_IMAGE_VERIFY:-0}" != "1" ]]; then
  printf '导入本机 Docker 并校验镜像架构...\n'
  docker load --input "${DIRECT_IMAGE_ARCHIVE}" >/dev/null
  image_platform="$(docker image inspect "${IMAGE_REF}" --format '{{.Os}}/{{.Architecture}}')"
  [[ "${image_platform}" == "linux/amd64" ]] || die "镜像架构为 ${image_platform}，不是 linux/amd64"
fi

(
  cd "${OUTPUT_ROOT}"
  shasum -a 256 "${IMAGE_ARCHIVE_NAME}" > "${IMAGE_ARCHIVE_NAME}.sha256"
)

printf '\n构建完成：\n'
printf '  镜像：%s\n' "${IMAGE_REF}"
printf '  可直接导入极空间 Docker 的镜像包：%s\n' "${DIRECT_IMAGE_ARCHIVE}"
printf '  校验文件：%s\n' "${IMAGE_CHECKSUM}"
printf '把 .docker.tar 导入极空间 Docker，只需映射 /data 并发布容器 8080 端口。\n'
