# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

# This Dockerfile contains multiple targets.
# Use 'docker build --target=<name> .' to build one.

# ===================================
#   Non-release images.
# ===================================

# devbuild compiles the binary
# -----------------------------------
FROM golang:1.25 AS devbuild

# Disable CGO to make sure we build static binaries
ENV CGO_ENABLED=0

# Escape the GOPATH
WORKDIR /build
COPY . ./
RUN go build -o nomad-autoscaler .

# dev runs the binary from devbuild
# -----------------------------------
FROM alpine:3.22 AS dev

COPY --from=devbuild /build/nomad-autoscaler /bin/
COPY ./scripts/docker-entrypoint.sh /

ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["help"]


# ===================================
#   Release images.
# ===================================

FROM alpine:3.22 AS release

ARG PRODUCT_NAME=nomad-autoscaler
ARG PRODUCT_VERSION
ARG PRODUCT_REVISION
# TARGETARCH and TARGETOS are set automatically when --platform is provided.
ARG TARGETOS TARGETARCH

LABEL maintainer="Nomad Team <nomad@hashicorp.com>" \
      version=${PRODUCT_VERSION} \
      revision=${PRODUCT_REVISION} \
      org.opencontainers.image.title="nomad-autoscaler" \
      org.opencontainers.image.description="Nomad Autoscaler is a horizontal application and cluster autoscaler for HashiCorp Nomad" \
      org.opencontainers.image.authors="Nomad Team <nomad@hashicorp.com>" \
      org.opencontainers.image.url="https://github.com/hashicorp/nomad-autoscaler" \
      org.opencontainers.image.documentation="https://developer.hashicorp.com/nomad/tools/autoscaling" \
      org.opencontainers.image.source="https://github.com/hashicorp/nomad-autoscaler" \
      org.opencontainers.image.version=${PRODUCT_VERSION} \
      org.opencontainers.image.revision=${PRODUCT_REVISION} \
      org.opencontainers.image.vendor="HashiCorp" \
      org.opencontainers.image.licenses="MPL-2.0"

RUN mkdir -p /usr/share/doc/nomad-autoscaler
COPY LICENSE /usr/share/doc/nomad-autoscaler/LICENSE.txt

COPY dist/$TARGETOS/$TARGETARCH/nomad-autoscaler /bin/
COPY ./scripts/docker-entrypoint.sh /

# Create a non-root user to run the software.
RUN addgroup $PRODUCT_NAME && \
    adduser -S -G $PRODUCT_NAME $PRODUCT_NAME

USER $PRODUCT_NAME
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["help"]

# ===================================
#   Set default target to 'dev'.
# ===================================
FROM dev
