# Build the commands by default
.DEFAULT_GOAL := all

# Executables
GO ?= $(shell which go 2>/dev/null)
NPM ?= $(shell which npm 2>/dev/null)
DOCKER ?= $(shell which docker 2>/dev/null)
GOWORK ?= off

# Locations
BUILD_DIR ?= build
CMD_DIR := $(wildcard cmd/*)
TS_DIR := frontend

# Set OS and Architecture
ARCH ?= $(shell arch | tr A-Z a-z | sed 's/x86_64/amd64/' | sed 's/i386/amd64/' | sed 's/armv7l/arm/' | sed 's/aarch64/arm64/')
OS ?= $(shell uname | tr A-Z a-z)
VERSION ?= $(shell git describe --tags --always | sed 's/^v//')

# Set test flags, such as -short, which skips the tests which call OpenAI's
# API, or an Ollama server
TEST_FLAGS ?=

# Set build flags
VERSION_PKG = github.com/mutablelogic/go-server/pkg/version
BUILD_LD_FLAGS += -X $(VERSION_PKG).GitTag=$(shell git describe --tags --always)
BUILD_LD_FLAGS += -X $(VERSION_PKG).GitBranch=$(shell git name-rev HEAD --name-only --always)
BUILD_FLAGS = -ldflags "-s -w ${BUILD_LD_FLAGS}"

###############################################################################
# ALL

.PHONY: all
all: build

###############################################################################
# BUILD

# Build the commands in the cmd directory, with the web UI, which the server
# embeds with the "frontend" tag, once it's built
.PHONY: build
build: tidy $(CMD_DIR)

$(CMD_DIR): go-dep mkdir tidy
	@echo Build $(notdir $@) GOOS=${OS} GOARCH=${ARCH}
	@GOWORK=${GOWORK} GOOS=${OS} GOARCH=${ARCH} ${GO} build -tags frontend ${BUILD_FLAGS} -o ${BUILD_DIR}/$(notdir $@) ./$@

# Build the web UI into build/frontend, which the server embeds, with its
# dependencies, which are installed from package-lock.json, once it's type
# checked, as esbuild doesn't check its types
.PHONY: frontend
frontend: npm-dep mkdir
	@echo Build frontend
	@cd $(TS_DIR) && ${NPM} ci --silent && npx tsc --noEmit -p . && node build.js

# Build the client, which is the llm command without the server, with the
# "client" tag. It's built as build/llm, in place of the server.
.PHONY: client
client: go-dep mkdir
	@echo Build llm client GOOS=${OS} GOARCH=${ARCH}
	@GOWORK=${GOWORK} GOOS=${OS} GOARCH=${ARCH} ${GO} build -tags client ${BUILD_FLAGS} -o ${BUILD_DIR}/llm ./cmd/llm

###############################################################################
# DOCKER

# Build the server's image, for Linux, with the web UI, tagged with the
# version, for the architecture, such as llm:1.2.3-linux-arm64
.PHONY: docker
docker: docker-dep
	@echo Build docker image llm:${VERSION}-linux-${ARCH}
	@${DOCKER} build --build-arg OS=linux --build-arg ARCH=${ARCH} --build-arg SOURCE=https://github.com/mutablelogic/go-llm \
		-f docker/Dockerfile -t llm:${VERSION}-linux-${ARCH} .

###############################################################################
# TEST

.PHONY: test
test: vet unit-test coverage-test

.PHONY: vet
vet: go-dep
	@echo Vet
	@GOWORK=${GOWORK} ${GO} vet ./...
	@GOWORK=${GOWORK} ${GO} vet -tags client ./...

.PHONY: unit-test
unit-test: go-dep
	@echo Unit Tests
	@GOWORK=${GOWORK} ${GO} test ${TEST_FLAGS} ./...

.PHONY: race-test
race-test: go-dep
	@echo Race Tests
	@GOWORK=${GOWORK} ${GO} test -race ${TEST_FLAGS} ./...

.PHONY: coverage-test
coverage-test: go-dep mkdir
	@echo Test Coverage
	@GOWORK=${GOWORK} ${GO} test ${TEST_FLAGS} -v -coverprofile ${BUILD_DIR}/coverprofile.out ./...
	@${GO} tool cover -func ${BUILD_DIR}/coverprofile.out > ${BUILD_DIR}/coverage.txt

###############################################################################
# CLEAN

# Other rules
.PHONY: mkdir
mkdir:
	@install -d $(BUILD_DIR)

# The web UI is built first, as the server's "frontend" tag imports it, which
# go mod tidy, which looks at every tag, needs
.PHONY: tidy
tidy: go-dep frontend
	@echo 'go tidy'
	@GOWORK=${GOWORK} $(GO) mod tidy

.PHONY: clean
clean:
	@echo 'clean'
	@rm -fr $(BUILD_DIR)
	@$(GO) clean

###############################################################################
# DEPENDENCIES

.PHONY: npm-dep
npm-dep:
	@test -f "${NPM}" && test -x "${NPM}"  || (echo "Missing npm binary, which builds the web UI" && exit 1)

.PHONY: docker-dep
docker-dep:
	@test -f "${DOCKER}" && test -x "${DOCKER}"  || (echo "Missing docker binary" && exit 1)

.PHONY: go-dep
go-dep:
	@test -f "${GO}" && test -x "${GO}"  || (echo "Missing go binary" && exit 1)
