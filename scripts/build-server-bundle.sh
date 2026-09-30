#!/usr/bin/env bash
set -euo pipefail

TAG="${1:-team}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PLATFORM="${WEKNORA_PLATFORM:-linux/amd64}"
OUT_DIR="${WEKNORA_IMAGE_OUT_DIR:-${ROOT_DIR}/dist/images}"
OUT_FILE="${OUT_DIR}/aiks-weknora-${TAG}.tar"

APP_IMAGE="${WEKNORA_APP_IMAGE_REPO:-aiks-weknora-app}:${TAG}"
FRONTEND_IMAGE="${WEKNORA_FRONTEND_IMAGE_REPO:-aiks-weknora-frontend}:${TAG}"
DOCREADER_IMAGE="${WEKNORA_DOCREADER_IMAGE_REPO:-aiks-weknora-docreader}:${TAG}"

POSTGRES_IMAGE="${WEKNORA_POSTGRES_IMAGE:-paradedb/paradedb:v0.22.6-pg17}"
REDIS_IMAGE="${WEKNORA_REDIS_IMAGE:-redis:7.0-alpine}"

# 国内 Docker Hub 镜像代理
DOCKER_MIRROR="${DOCKER_MIRROR:-docker.1ms.run}"

BASE_IMAGES=(
  "debian:12.12-slim"
  "golang:1.26-bookworm"
  "node:24-bookworm-slim"
  "python:3.10.18-bookworm"
  "rust:1.98.1-bookworm"
  "nginx:1.30.3-alpine"
  "$POSTGRES_IMAGE"
  "$REDIS_IMAGE"
)

mkdir -p "$OUT_DIR"

echo "[INFO] target platform: $PLATFORM"
echo "[INFO] Docker mirror: $DOCKER_MIRROR"

#
# 从国内镜像源拉取，然后重新 tag 成 Dockerfile 使用的原始名称。
#
pull_from_mirror() {
  local image="$1"
  local mirror_image="${DOCKER_MIRROR}/${image}"

  echo "[INFO] pulling: $mirror_image"

  docker pull \
    --platform "$PLATFORM" \
    "$mirror_image"

  echo "[INFO] tagging: $mirror_image -> $image"

  docker tag \
    "$mirror_image" \
    "$image"
}

echo "[INFO] pulling base/dependency images from domestic mirror"

for base in "${BASE_IMAGES[@]}"; do
  pull_from_mirror "$base"
done


echo "[INFO] building $APP_IMAGE using local base images"

docker build \
  --platform "$PLATFORM" \
  --pull=false \
  --build-arg APK_MIRROR_ARG="${APK_MIRROR_ARG:-mirrors.aliyun.com}" \
  --build-arg GOPROXY_ARG="${GOPROXY_ARG:-https://goproxy.cn,direct}" \
  --build-arg WITH_ANYDOC="${WITH_ANYDOC:-1}" \
  --build-arg WITH_LICENSE_BUNDLE="${WITH_LICENSE_BUNDLE:-0}" \
  -t "$APP_IMAGE" \
  -f "${ROOT_DIR}/docker/Dockerfile.app" \
  "$ROOT_DIR"


echo "[INFO] building $FRONTEND_IMAGE using local base images"

docker build \
  --platform "$PLATFORM" \
  --pull=false \
  --build-arg VITE_FRONTEND_COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)" \
  --build-arg NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmmirror.com}" \
  -t "$FRONTEND_IMAGE" \
  "${ROOT_DIR}/frontend"


echo "[INFO] building $DOCREADER_IMAGE using local base images"

docker build \
  --platform "$PLATFORM" \
  --pull=false \
  --build-arg APT_MIRROR="${APT_MIRROR:-http://mirrors.aliyun.com}" \
  -t "$DOCREADER_IMAGE" \
  -f "${ROOT_DIR}/docker/Dockerfile.docreader" \
  "$ROOT_DIR"


echo "[INFO] saving one offline bundle: $OUT_FILE"

docker save -o "$OUT_FILE" \
  "$APP_IMAGE" \
  "$FRONTEND_IMAGE" \
  "$DOCREADER_IMAGE" \
  "$POSTGRES_IMAGE" \
  "$REDIS_IMAGE"


echo
echo "[OK] offline image bundle generated:"
echo "     $OUT_FILE"
echo
echo "Upload it to:"
echo "     deploy/server/images/"