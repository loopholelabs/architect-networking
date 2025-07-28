FROM fedora:41

ARG GO_VERSION=1.24.2

RUN dnf update -y
RUN dnf install -y git wget make automake tar
RUN dnf install -y pkg-config clang llvm m4
RUN dnf install -y iproute ethtool protobuf-compiler
RUN (test $(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) = "arm64" && dnf install -y libbpf-devel glibc-devel) || true
RUN (test $(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/) = "amd64" && dnf install -y libbpf-devel glibc-devel glibc-devel.i686) || true

RUN wget "https://go.dev/dl/go${GO_VERSION}.linux-$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/).tar.gz"
RUN rm -rf /usr/local/go && tar -C /usr/local -xzf go${GO_VERSION}.linux-$(arch | sed s/aarch64/arm64/ | sed s/x86_64/amd64/).tar.gz
RUN mkdir -p /root/go

ENV PATH="$PATH:/usr/local/go/bin:/root/go/bin"
ENV GOPATH=/root/go

RUN go install github.com/loopholelabs/frpc-go/protoc-gen-go-frpc@latest

RUN mkdir -p /root/architect-networking
WORKDIR /root/architect-networking
COPY go.mod .
COPY go.sum .
RUN go mod download