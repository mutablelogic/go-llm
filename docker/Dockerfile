ARG OS=linux
ARG ARCH=amd64
ARG GO_VERSION=1.27

# Build the commands on the Go image, without cgo, so they're static
FROM --platform=${OS}/${ARCH} golang:${GO_VERSION}-trixie AS builder
ARG OS
ARG ARCH
WORKDIR /usr/src/app

# The server embeds the web UI, which is built with Node
RUN set -x && apt-get update -y \
    && apt-get install -y --no-install-recommends nodejs npm \
    && rm -rf /var/lib/apt/lists/*

# Download the modules first, so they're only downloaded again when they
# change
COPY go.mod go.sum ./
RUN go mod download

# Build the commands, with the web UI
COPY . .
RUN set -x \
    && OS=${OS} ARCH=${ARCH} CGO_ENABLED=0 make all \
    && install -m 0755 docker/entrypoint.sh build/entrypoint.sh

# Copy the command to /usr/local/bin, on Debian, with the CA certificates
# providers' APIs are called with, and gosu, which the entrypoint drops
# privileges with, to the llm user, which the server runs as
FROM --platform=${OS}/${ARCH} debian:trixie-slim
ARG SOURCE
RUN set -x && apt-get update -y \
    && apt-get install -y --no-install-recommends ca-certificates gosu \
    && groupadd --system llm \
    && useradd --system --gid llm --no-create-home --shell /usr/sbin/nologin llm \
    && install -d -o llm -g llm /data \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /usr/src/app/build/llm /usr/src/app/build/entrypoint.sh /usr/local/bin/

# Check the command runs
RUN /usr/local/bin/llm --version

# Label the image
LABEL org.opencontainers.image.source=${SOURCE}

# The server keeps its providers, sessions, agents and memory in /data,
# which is its cache directory, and listens on every interface. The
# entrypoint adds the providers of a new installation: Eliza, unless NO_ELIZA
# is set, and those whose variables are set, such as OLLAMA_ADDR, or
# OPENAI_API_KEY.
ENV XDG_CACHE_HOME=/data

# The server listens on every interface in the container, as it must for
# Docker to forward a published port to it. Which interfaces of the host
# reach it is how the port is published: the server has no authentication
# yet, so publish it on localhost only, with -p 127.0.0.1:8084:8084, rather
# than -p 8084:8084, which is every interface, unless it's behind a proxy
# which signs people in, and never on the internet.
ENV ADDR=0.0.0.0:8084
VOLUME /data
EXPOSE 8084
STOPSIGNAL SIGTERM

# Entrypoint runs as root, optionally remaps the llm user's UID and GID,
# makes /data the llm user's, then drops to the llm user before exec-ing llm
ENTRYPOINT [ "/usr/local/bin/entrypoint.sh" ]
