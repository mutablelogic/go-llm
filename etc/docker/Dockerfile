ARG OS
ARG ARCH

# Build stage
FROM --platform=${OS}/${ARCH} golang:1.25 AS builder
ARG OS
ARG ARCH
WORKDIR /usr/src/app
COPY . .

RUN OS=${OS} ARCH=${ARCH} make build

# Runtime stage
FROM --platform=${OS}/${ARCH} debian:trixie-slim
ARG SOURCE
RUN apt-get update -y && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=builder /usr/src/app/build/llm /usr/local/bin/llm

# Labels
LABEL org.opencontainers.image.source=https://${SOURCE}

# Entrypoint
ENV LLM_ADDR=:8085
ENV XDG_CACHE_HOME=/data
VOLUME /data
EXPOSE 8085
ENTRYPOINT ["/usr/local/bin/llm"]
