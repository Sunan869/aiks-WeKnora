# Build extension and daemon from the same pinned source on the runtime architecture.
FROM node:24-bookworm-slim AS browserskill
FROM rust:1.98.1-bookworm AS rusttoolchain

ARG APK_MIRROR_ARG

WORKDIR /build

# Debian 国内镜像
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
    sed -i \
    "s@deb.debian.org@$APK_MIRROR_ARG@g; \
    s@security.debian.org@$APK_MIRROR_ARG@g" \
    /etc/apt/sources.list.d/debian.sources 2>/dev/null || true; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    git \
    python3 \
    ca-certificates \
    curl \
    build-essential \
    cmake \
    pkg-config && \
    rm -rf /var/lib/apt/lists/*

# Rust 国内镜像
ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    RUSTUP_DIST_SERVER=https://rsproxy.cn \
    RUSTUP_UPDATE_ROOT=https://rsproxy.cn/rustup \
    PATH=/usr/local/cargo/bin:$PATH

# 配置 Cargo 国内 crates.io 镜像
RUN mkdir -p /usr/local/cargo && \
    printf '%s\n' \
    '[source.crates-io]' \
    'replace-with = "rsproxy-sparse"' \
    '' \
    '[source.rsproxy-sparse]' \
    'registry = "sparse+https://rsproxy.cn/index/"' \
    '' \
    '[net]' \
    'git-fetch-with-cli = true' \
    > /usr/local/cargo/config.toml

# 不再访问 sh.rustup.rs
# 先下载再执行，curl 失败时 Docker 构建会直接失败
RUN curl \
    --retry 5 \
    --retry-delay 2 \
    --connect-timeout 20 \
    --proto '=https' \
    --tlsv1.2 \
    -fsSL \
    https://rsproxy.cn/rustup-init.sh \
    -o /tmp/rustup-init.sh && \
    sh /tmp/rustup-init.sh \
    -y \
    --profile minimal \
    --default-toolchain stable && \
    rm -f /tmp/rustup-init.sh && \
    rustc --version && \
    cargo --version

COPY scripts/build_browserskill.sh scripts/browserskill-release.json ./scripts/

ARG TARGETOS
ARG TARGETARCH

RUN bash scripts/build_browserskill.sh \
    /opt/weknora/browserskill \
    "${TARGETOS}/${TARGETARCH}"


# ============================================================
# Build stage
# ============================================================

FROM golang:1.26-bookworm AS builder

WORKDIR /app

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    PATH=/usr/local/cargo/bin:$PATH

COPY --from=rusttoolchain /usr/local/rustup /usr/local/rustup
COPY --from=rusttoolchain /usr/local/cargo /usr/local/cargo

# 通过构建参数接收敏感信息
ARG GOPRIVATE_ARG
ARG GOPROXY_ARG=https://goproxy.cn,direct
ARG GOSUMDB_ARG=off
ARG APK_MIRROR_ARG

# 设置 Go 环境变量
ENV GOPRIVATE=${GOPRIVATE_ARG} \
    GOPROXY=${GOPROXY_ARG} \
    GOSUMDB=${GOSUMDB_ARG}

# Install dependencies
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
    sed -i \
    "s@deb.debian.org@${APK_MIRROR_ARG}@g; \
    s@security.debian.org@${APK_MIRROR_ARG}@g" \
    /etc/apt/sources.list.d/debian.sources 2>/dev/null || true; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    git \
    build-essential \
    libsqlite3-dev \
    curl \
    ca-certificates && \
    rm -rf /var/lib/apt/lists/*


# Install migrate tool
RUN --mount=type=cache,target=/go/pkg/mod \
    go install -tags 'postgres' \
    github.com/golang-migrate/migrate/v4/cmd/migrate@latest


# Copy go mod files.
# go.mod replace-points anydoc at ./third_party/anydoc-go,
# so that module's go.mod must exist before `go mod download`.
COPY go.mod go.sum ./
COPY third_party/anydoc-go/go.mod third_party/anydoc-go/go.mod

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd/download cmd/download

RUN --mount=type=cache,target=/go/pkg/mod \
    go run cmd/download/duckdb/duckdb.go

COPY . .


ARG WITH_LICENSE_BUNDLE=1

RUN --mount=type=cache,target=/go/pkg/mod \
    if [ "$WITH_LICENSE_BUNDLE" = "1" ]; then \
    bash ./scripts/copy-licenses.sh /license-bundle; \
    else \
    mkdir -p /license-bundle/licenses/sources && \
    cp LICENSE THIRD_PARTY_NOTICES.md /license-bundle/; \
    fi


# Get version and commit info for build injection
ARG VERSION_ARG
ARG COMMIT_ID_ARG
ARG BUILD_TIME_ARG
ARG GO_VERSION_ARG

# Set build-time variables
ENV VERSION=${VERSION_ARG} \
    COMMIT_ID=${COMMIT_ID_ARG} \
    BUILD_TIME=${BUILD_TIME_ARG} \
    GO_VERSION=${GO_VERSION_ARG}


# ============================================================
# AnyDoc / Rust
# ============================================================

# Link the anydoc parser engine (office docs converted in-process, no
# Python docreader). Default on so Hub / compose images ship a working
# engine; pass WITH_ANYDOC=0 to skip the Rust toolchain.
ARG WITH_ANYDOC=1

ENV RUSTUP_HOME=/usr/local/rustup \
    CARGO_HOME=/usr/local/cargo \
    RUSTUP_DIST_SERVER=https://rsproxy.cn \
    RUSTUP_UPDATE_ROOT=https://rsproxy.cn/rustup \
    PATH=/usr/local/cargo/bin:$PATH


# Cargo 国内源
RUN mkdir -p /usr/local/cargo && \
    printf '%s\n' \
    '[source.crates-io]' \
    'replace-with = "rsproxy-sparse"' \
    '' \
    '[source.rsproxy-sparse]' \
    'registry = "sparse+https://rsproxy.cn/index/"' \
    '' \
    '[net]' \
    'git-fetch-with-cli = true' \
    > /usr/local/cargo/config.toml


# 安装 Rust + 编译 AnyDoc
RUN --mount=type=cache,target=/usr/local/cargo/registry \
    --mount=type=cache,target=/usr/local/cargo/git \
    if [ "$WITH_ANYDOC" = "1" ]; then \
    rustc --version && \
    cargo --version && \
    bash ./scripts/build-anydoc-lib.sh; \
    fi


# Build the application with version info
RUN --mount=type=cache,target=/go/pkg/mod \
    if [ "$WITH_ANYDOC" = "1" ]; then \
    make build-prod GO_BUILD_TAGS=anydoc; \
    else \
    make build-prod; \
    fi

RUN --mount=type=cache,target=/go/pkg/mod \
    cp -r /go/pkg/mod/github.com/yanyiwu/ /app/yanyiwu/


# ============================================================
# Final stage
# ============================================================

FROM debian:12.12-slim

WORKDIR /app

ARG APK_MIRROR_ARG
ARG PYPI_MIRROR=https://mirrors.aliyun.com/pypi/simple/


# Pairing derives the gateway URL from the user's page origin by default.
ENV BROWSERSKILL_BINARY=/opt/weknora/browserskill/bsk \
    BROWSERSKILL_EXTENSION_PATH=/opt/weknora/browserskill/browser-skill-weknora-0.3.1.zip

COPY --from=browserskill \
    /opt/weknora/browserskill \
    /opt/weknora/browserskill


# Create a non-root user first
RUN useradd -m -s /bin/bash appuser


# Install runtime dependencies
RUN if [ -n "$APK_MIRROR_ARG" ]; then \
    sed -i \
    "s@deb.debian.org@$APK_MIRROR_ARG@g; \
    s@security.debian.org@$APK_MIRROR_ARG@g" \
    /etc/apt/sources.list.d/debian.sources 2>/dev/null || true; \
    fi && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    ca-certificates \
    build-essential \
    postgresql-client \
    default-mysql-client \
    tzdata \
    sed \
    curl \
    bash \
    vim \
    wget \
    libsqlite3-0 \
    python3 \
    python3-pip \
    python3-dev \
    libffi-dev \
    libssl-dev \
    nodejs \
    npm \
    gosu \
    ffmpeg && \
    rm -rf /var/lib/apt/lists/*


# Python / uv 使用国内 PyPI 源
#
# 不再：
# curl https://astral.sh/uv/install.sh
#
# 避免国内网络访问 astral.sh / GitHub 出现问题。
RUN python3 -m pip install \
    --break-system-packages \
    --no-cache-dir \
    --index-url "${PYPI_MIRROR}" \
    --upgrade \
    pip \
    setuptools \
    wheel && \
    python3 -m pip install \
    --break-system-packages \
    --no-cache-dir \
    --index-url "${PYPI_MIRROR}" \
    uv && \
    uv --version && \
    uvx --version


# Create data directories and set permissions
RUN mkdir -p /data/files /home/appuser/.local/bin && \
    chown -R appuser:appuser /app /data/files /home/appuser


# Copy migrate tool from builder stage
COPY --from=builder /go/bin/migrate /usr/local/bin/

COPY --from=builder \
    /app/yanyiwu/ \
    /go/pkg/mod/github.com/yanyiwu/


# Copy the binary from the builder stage
COPY --from=builder /app/config ./config
COPY --from=builder /app/scripts ./scripts
COPY --from=builder /app/migrations ./migrations
COPY --from=builder /app/dataset/samples ./dataset/samples
COPY --from=builder /root/.duckdb /home/appuser/.duckdb
COPY --from=builder /app/WeKnora .
COPY --from=builder /license-bundle/ ./


# Copy and make entrypoint script executable
COPY --from=builder \
    /app/scripts/docker-entrypoint.sh \
    ./scripts/docker-entrypoint.sh


# Make scripts executable
RUN chmod +x ./scripts/*.sh


# Expose ports
EXPOSE 8080


ENTRYPOINT ["./scripts/docker-entrypoint.sh"]
CMD ["./WeKnora"]