FROM gcr.io/distroless/static-debian12
COPY build/arc-nat /usr/bin/arc-nat
USER 65534:65534
ENTRYPOINT ["/usr/bin/arc-nat"]