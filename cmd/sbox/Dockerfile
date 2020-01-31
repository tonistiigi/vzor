#syntax=docker/dockerfile:experimental

FROM golang:1.13-alpine AS sbox
RUN apk add --no-cache file git
WORKDIR /src
ENV CGO_ENABLED=0
RUN --mount=target=. \
  --mount=target=/root/.cache,type=cache \
  --mount=target=/go/pkg/mod,type=cache \
  go build -o /out/sbox ./cmd/sbox && \
  file /out/sbox | egrep "statically linked|Mach-O|Windows"

FROM alpine
RUN apk add --no-cache shadow
RUN useradd --create-home --home-dir /home/user --uid 1000 -s /bin/sh user
COPY --chown=1000:1000 --from=alpine / /demo/rootfs/
RUN echo nameserver 8.8.8.8 > /demo/rootfs/etc/resolv.conf && chown 1000:1000 /demo/rootfs/etc/resolv.conf
COPY --from=sbox /out/sbox /bin/
WORKDIR /demo
USER user
