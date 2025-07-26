DOCKER_IMAGE = architect-networking-build:local
DEFAULT_BUILD_ARGS = '-ldflags=-s -w' -trimpath

.PHONY: build-image
build-image:
	 docker build --tag $(DOCKER_IMAGE) --build-arg GITHUB_TOKEN=${GITHUB_TOKEN} . -f build.Dockerfile

.PHONY: generate
generate:
	docker run --rm -v .:/root/architect-networking --privileged $(DOCKER_IMAGE) bash -c "go generate ./..."

.PHONY: build
build: generate
	docker run --rm -v .:/root/architect-networking --privileged $(DOCKER_IMAGE) bash -c "go build $(DEFAULT_BUILD_ARGS) -o build/arc-nat cmd/main.go"