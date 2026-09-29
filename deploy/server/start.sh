#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT_DIR"

COMPOSE=(docker compose --env-file .env -f docker-compose.yml)

die() {
  echo "[ERROR] $*" >&2
  exit 1
}

env_value() {
  local key="$1" line value
  line="$(grep -m1 -E "^${key}=" .env 2>/dev/null || true)"
  value="${line#*=}"
  printf '%s' "$value" | tr -d '\r'
}

set_env() {
  local key="$1" value="$2" tmp
  tmp=".env.tmp.$"
  if grep -qE "^${key}=" .env; then
    awk -v key="$key" -v value="$value" '
      BEGIN { prefix = key "=" }
      index($0, prefix) == 1 { print prefix value; next }
      { print }
    ' .env > "$tmp"
    mv "$tmp" .env
  else
    printf '%s=%s\n' "$key" "$value" >> .env
  fi
}

random_hex() {
  local bytes="$1"
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex "$bytes"
  else
    od -An -N"$bytes" -tx1 /dev/urandom | tr -d ' \n'
  fi
}

load_images() {
  mkdir -p images
  local found=0 file
  shopt -s nullglob
  for file in images/*.tar; do
    found=1
    echo "[INFO] docker load: $file"
    docker load -i "$file"
  done
  for file in images/*.tar.gz images/*.tgz; do
    found=1
    echo "[INFO] docker load: $file"
    gzip -dc "$file" | docker load
  done
  shopt -u nullglob
  if [ "$found" -eq 0 ]; then
    echo "[INFO] images/ has no image archive; using already loaded local images."
  fi
}

ensure_env() {
  if [ ! -f .env ]; then
    cp .env.example .env
    echo "[INFO] created .env from .env.example"
  fi
  [ -n "$(env_value DB_PASSWORD)" ] || set_env DB_PASSWORD "$(random_hex 24)"
  [ -n "$(env_value REDIS_PASSWORD)" ] || set_env REDIS_PASSWORD "$(random_hex 24)"
  [ -n "$(env_value JWT_SECRET)" ] || set_env JWT_SECRET "$(random_hex 32)"
  # SYSTEM_AES_KEY is exactly 32 ASCII bytes.
  [ -n "$(env_value SYSTEM_AES_KEY)" ] || set_env SYSTEM_AES_KEY "$(random_hex 16)"
  [ -n "$(env_value SYSTEM_SIGNING_KEY)" ] || set_env SYSTEM_SIGNING_KEY "$(random_hex 32)"

  echo "[INFO] deployment secrets are present in .env"
}

ensure_network() {
  local network
  network="$(env_value AIKS_TEAM_NETWORK)"
  network="${network:-aiks-team-network}"
  if ! docker network inspect "$network" >/dev/null 2>&1; then
    docker network create "$network" >/dev/null
    echo "[INFO] created docker network: $network"
  fi
}

require_images() {
  local app_image frontend_image docreader_image postgres_image redis_image
  app_image="$(env_value WEKNORA_APP_IMAGE)"; app_image="${app_image:-aiks-weknora-app:team}"
  frontend_image="$(env_value WEKNORA_FRONTEND_IMAGE)"; frontend_image="${frontend_image:-aiks-weknora-frontend:team}"
  docreader_image="$(env_value WEKNORA_DOCREADER_IMAGE)"; docreader_image="${docreader_image:-aiks-weknora-docreader:team}"
  postgres_image="$(env_value WEKNORA_POSTGRES_IMAGE)"; postgres_image="${postgres_image:-paradedb/paradedb:v0.22.6-pg17}"
  redis_image="$(env_value WEKNORA_REDIS_IMAGE)"; redis_image="${redis_image:-redis:7.0-alpine}"
  local images=("$app_image" "$frontend_image" "$docreader_image" "$postgres_image" "$redis_image")
  local image
  for image in "${images[@]}"; do
    docker image inspect "$image" >/dev/null 2>&1 ||       die "missing image: $image. Put the bundle tar under images/ and rerun ./start.sh."
  done
}

start_service() {
  ensure_env
  load_images
  ensure_network
  require_images
  "${COMPOSE[@]}" config >/dev/null
  "${COMPOSE[@]}" up -d --force-recreate --remove-orphans

  local app frontend_port app_bind app_port
  app="$(env_value WEKNORA_APP_CONTAINER_NAME)"; app="${app:-weknora-app}"
  frontend_port="$(env_value WEKNORA_FRONTEND_PORT)"; frontend_port="${frontend_port:-80}"
  app_bind="$(env_value WEKNORA_APP_BIND)"; app_bind="${app_bind:-127.0.0.1}"
  app_port="$(env_value WEKNORA_APP_PORT)"; app_port="${app_port:-8080}"
  echo "[INFO] waiting for WeKnora app health..."
  for _ in $(seq 1 60); do
    if docker exec "$app" curl -fsS http://127.0.0.1:8080/health >/dev/null 2>&1; then
      echo "[OK] aiks-WeKnora is healthy."
      "${COMPOSE[@]}" ps
      echo "[INFO] frontend: http://<server>:$frontend_port"
      echo "[INFO] app debug endpoint: $app_bind:$app_port"
      return 0
    fi
    sleep 2
  done

  "${COMPOSE[@]}" ps
  docker logs --tail 160 "$app" || true
  die "WeKnora health check timed out"
}

case "${1:-start}" in
  start|up) start_service ;;
  restart)
    "${COMPOSE[@]}" down
    start_service
    ;;
  stop|down) "${COMPOSE[@]}" down ;;
  status|ps) "${COMPOSE[@]}" ps ;;
  logs) "${COMPOSE[@]}" logs -f --tail=200 ;;
  *)
    echo "Usage: $0 {start|restart|stop|status|logs}" >&2
    exit 2
    ;;
esac
