FROM cgr.dev/chainguard/wolfi-base:latest
COPY build/arc-net /usr/bin/arc-net
USER 65534:65534
ENTRYPOINT ["/usr/bin/arc-net"]