BUILD_DOCKER_IMAGE = architect-networking-build:local
RUN_DOCKER_IMAGE = architect-networking:local
BUILD_GIT_COMMIT = $(shell git rev-parse --short HEAD)
BUILD_GO_VERSION = $(shell (go version | awk '{print $$3}'))
BUILD_DATE=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
BUILD_PLATFORM = $(shell (go version | awk '{print $$4}'))
DEFAULT_BUILD_ARGS = -ldflags='-s -w -X github.com/loopholelabs/architect-networking/version.GitCommit=$(BUILD_GIT_COMMIT) -X github.com/loopholelabs/architect-networking/version.GoVersion=$(BUILD_GO_VERSION) -X github.com/loopholelabs/architect-networking/version.BuildDate=$(BUILD_DATE) -X github.com/loopholelabs/architect-networking/version.Platform=$(BUILD_PLATFORM)' -trimpath

.PHONY: build-image
build-image:
	 docker build --tag $(BUILD_DOCKER_IMAGE) . -f build.Dockerfile

.PHONY: run-image
run-image:
	 docker build --tag $(RUN_DOCKER_IMAGE) . -f run.Dockerfile

.PHONY: generate
generate:
	docker run --rm -v .:/root/architect-networking --privileged $(BUILD_DOCKER_IMAGE) bash -c "go generate ./..."

.PHONY: build
build: generate
	docker run --rm -v .:/root/architect-networking --privileged $(BUILD_DOCKER_IMAGE) bash -c "go build $(DEFAULT_BUILD_ARGS) -o build/arc-nat cmd/main.go"