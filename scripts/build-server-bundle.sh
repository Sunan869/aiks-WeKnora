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
POSTGRES_SOURCE="${WEKNORA_POSTGRES_SOURCE:-docker.1ms.run/paradedb/paradedb:v0.22.6-pg17}"
REDIS_IMAGE="${WEKNORA_REDIS_IMAGE:-redis:7.0-alpine}"
REDIS_SOURCE="${WEKNORA_REDIS_SOURCE:-docker.1ms.run/redis:7.0-alpine}"

mkdir -p "$OUT_DIR"

echo "[INFO] target platform: $PLATFORM"

echo "[INFO] building $APP_IMAGE"
docker buildx build --platform "$PLATFORM" --load   --build-arg APK_MIRROR_ARG="${APK_MIRROR_ARG:-mirrors.aliyun.com}"   --build-arg GOPROXY_ARG="${GOPROXY_ARG:-https://goproxy.cn,direct}"   --build-arg WITH_ANYDOC="${WITH_ANYDOC:-1}"   -t "$APP_IMAGE"   -f "${ROOT_DIR}/docker/Dockerfile.app"   "$ROOT_DIR"

echo "[INFO] building $FRONTEND_IMAGE"
docker buildx build --platform "$PLATFORM" --load   --build-arg VITE_FRONTEND_COMMIT="$(git -C "$ROOT_DIR" rev-parse --short HEAD 2>/dev/null || echo unknown)"   --build-arg NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmmirror.com}"   -t "$FRONTEND_IMAGE"   "${ROOT_DIR}/frontend"

echo "[INFO] building $DOCREADER_IMAGE"
docker buildx build --platform "$PLATFORM" --load   --build-arg APT_MIRROR="${APT_MIRROR:-http://mirrors.aliyun.com}"   -t "$DOCREADER_IMAGE"   -f "${ROOT_DIR}/docker/Dockerfile.docreader"   "$ROOT_DIR"

if ! docker image inspect "$POSTGRES_IMAGE" >/dev/null 2>&1; then
  echo "[INFO] pulling dependency $POSTGRES_SOURCE"
  docker pull --platform "$PLATFORM" "$POSTGRES_SOURCE"
  docker tag "$POSTGRES_SOURCE" "$POSTGRES_IMAGE"
fi

if ! docker image inspect "$REDIS_IMAGE" >/dev/null 2>&1; then
  echo "[INFO] pulling dependency $REDIS_SOURCE"
  docker pull --platform "$PLATFORM" "$REDIS_SOURCE"
  docker tag "$REDIS_SOURCE" "$REDIS_IMAGE"
fi

echo "[INFO] saving one offline bundle: $OUT_FILE"
docker save -o "$OUT_FILE"   "$APP_IMAGE"   "$FRONTEND_IMAGE"   "$DOCREADER_IMAGE"   "$POSTGRES_IMAGE"   "$REDIS_IMAGE"

echo "[OK] $OUT_FILE"
echo "Upload it to: deploy/server/images/"
