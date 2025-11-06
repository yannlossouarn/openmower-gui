# ---------- Go build (unchanged logic, slightly hardened)
FROM golang:1.21 AS build-go
COPY . /app
WORKDIR /app
# Copy only what Go needs first (better caching if you add go.mod/sum)
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest and build
COPY . .
# Produce a static binary (no CGO) for simpler runtime
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/openmower-gui

# ---------- Web build (modern, deterministic)
# Prefer Node 20 (Node 18 is maintenance and many libs require >=20)
FROM node:20-alpine AS build-web
WORKDIR /web

# Yarn via Corepack; pin a version so CI/local match
RUN corepack enable && corepack prepare yarn@4.5.1 --activate

# Copy manifests first for caching
COPY web/package.json web/yarn.lock ./ 
# If the repo uses Yarn Berry, include these:
# COPY web/.yarnrc.yml ./
# COPY web/.yarn/ .yarn/

# Deterministic install:
#  - Berry:   --immutable
#  - Classic: --frozen-lockfile   (uncomment the right one)
RUN yarn install --immutable
# RUN yarn install --frozen-lockfile

# Copy the remaining sources
COPY web/. .

# If your build reads env at build time (Vite = VITE_*), provide safe defaults
# ARG VITE_MAP_TILE_SERVER="https://tiles.example.com"
# ENV VITE_MAP_TILE_SERVER=$VITE_MAP_TILE_SERVER

# Avoid “warnings as errors” behaviour inside container builds
ENV CI=false
RUN yarn build

# ---------- Tools layer (kept, but slimmed a bit and ordered)
FROM ubuntu:22.04 AS deps
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates curl git build-essential unzip wget autoconf automake \
    pkg-config texinfo libtool libjim-dev libftdi-dev libusb-1.0-0-dev \
    python3 python3-pip python3-venv \
    && rm -rf /var/lib/apt/lists/*

# Best-effort install for Pi GPIO meta (won't fail build)
RUN apt-get update && apt-get install -y --no-install-recommends rpi.gpio-common || true && rm -rf /var/lib/apt/lists/*

# Build OpenOCD with Raspberry Pi options
RUN git clone --recursive --branch rpi-common --depth=1 https://github.com/raspberrypi/openocd.git && \
    cd openocd && ./bootstrap with-submodules && \
    ./configure --enable-ftdi --enable-sysfsgpio --enable-bcm2835gpio && \
    make -j"$(nproc)" && make install && \
    cd .. && rm -rf openocd

# Install PlatformIO for CLI usage
RUN curl -fsSL https://raw.githubusercontent.com/platformio/platformio-core-installer/master/get-platformio.py -o get-platformio.py && \
    python3 get-platformio.py && \
    python3 -m pip install --no-cache-dir --upgrade pygnssutils && \
    mkdir -p /usr/local/bin && \
    ln -s ~/.platformio/penv/bin/platformio /usr/local/bin/platformio && \
    ln -s ~/.platformio/penv/bin/pio /usr/local/bin/pio && \
    ln -s ~/.platformio/penv/bin/piodebuggdb /usr/local/bin/piodebuggdb

# ---------- Final image
FROM deps
WORKDIR /app

# Web assets + Go binary from previous stages
COPY --from=build-web /web/dist /app/web
COPY --from=build-go  /out/openmower-gui /app/openmower-gui

ENV WEB_DIR=/app/web
ENV DB_PATH=/app/db

# Non-root is safer; comment out if tools require root
# RUN useradd -m -u 10001 appuser
# USER appuser

CMD ["/app/openmower-gui"]
