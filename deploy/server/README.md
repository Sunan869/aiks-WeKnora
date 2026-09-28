# aiks-WeKnora image-only server deployment

This is the recommended server package for the AIKS team architecture. The
server does not need the source repository and does not build images.

## 1. Build one image bundle locally

On the machine that has the source code:

```bash
./scripts/build-server-bundle.sh team
```

Default target platform is `linux/amd64`. For ARM64:

```bash
WEKNORA_PLATFORM=linux/arm64 ./scripts/build-server-bundle.sh team
```

The script builds the AIKS WeKnora app/frontend/docreader images, includes the
PostgreSQL and Redis dependency images, and writes one offline tar:

```text
dist/images/aiks-weknora-team.tar
```

## 2. Upload to server

Upload this `deploy/server` directory and place the tar under:

```text
deploy/server/images/aiks-weknora-team.tar
```

## 3. Start

```bash
cd deploy/server
cp .env.example .env
vi .env
chmod +x start.sh
./start.sh
```

On first start the script automatically generates DB/Redis/JWT/AES/signing
secrets when their values are empty, loads all image archives, creates the
shared `aiks-team-network`, force-recreates the containers and waits for the
WeKnora app health endpoint.

The public UI defaults to port 80. The app API defaults to loopback
`127.0.0.1:8080`. Put Nginx/TongHttpServer in front when HTTPS/domain access
is required.

## DingTalk

Fill these values in `.env` before enabling DingTalk:

- `DINGTALK_AUTH_CLIENT_ID`
- `DINGTALK_AUTH_CLIENT_SECRET`
- `DINGTALK_AUTH_CORP_ID`
- `DINGTALK_AUTH_DEPARTMENT_ORGANIZATION_MAPPINGS`

Then set:

```env
DINGTALK_AUTH_ENABLE=true
DINGTALK_AUTH_SYNC_DEPARTMENTS=true
```

The reverse proxy must preserve `Host` and set `X-Forwarded-Proto https` so
the backend generates the correct DingTalk callback URL.

## Upgrade

Replace the tar under `images/`, update the image tags in `.env` when
needed, then run:

```bash
./start.sh restart
```

Database, Redis, uploaded files and DocReader shared files use persistent Docker
volumes and are not deleted by a normal restart.
