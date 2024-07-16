# syntax=docker/dockerfile:1

# We use the latest Go 1.x version unless asked to use something else.
# The GitHub Actions CI job sets this argument for a consistent Go version.
ARG GO_VERSION=1

# Setup the base environment. The BUILDPLATFORM is set automatically by Docker.
# The --platform=${BUILDPLATFORM} flag tells Docker to build the function using
# the OS and architecture of the host running the build, not the OS and
# architecture that we're building the function for.
FROM debian:12.1-slim as package-stage

# TODO(negz): Use a proper Crossplane package building tool. We're abusing the
# fact that this image won't have an io.crossplane.pkg: base annotation. This
# means Crossplane package manager will pull this entire ~100MB image, which
# also happens to contain a valid Function runtime.
# https://github.com/crossplane/crossplane/blob/v1.13.2/contributing/specifications/xpkg.md
WORKDIR /package
COPY package/ ./

RUN cat crossplane.yaml > /package.yaml
RUN cat input/*.yaml >> /package.yaml

FROM --platform=${BUILDPLATFORM} golang:${GO_VERSION} AS build

WORKDIR /fn

# Most functions don't want or need CGo support, so we disable it.
ENV CGO_ENABLED=0

# We run go mod download in a separate step so that we can cache its results.
# This lets us avoid re-downloading modules if we don't need to. The type=target
# mount tells Docker to mount the current directory read-only in the WORKDIR.
# The type=cache mount tells Docker to cache the Go modules cache across builds.
RUN --mount=target=. --mount=type=cache,target=/go/pkg/mod go mod download

# The TARGETOS and TARGETARCH args are set by docker. We set GOOS and GOARCH to
# these values to ask Go to compile a binary for these architectures. If
# TARGETOS and TARGETOS are different from BUILDPLATFORM, Go will cross compile
# for us (e.g. compile a linux/amd64 binary on a linux/arm64 build machine).
ARG TARGETOS
ARG TARGETARCH

# Build the function binary. The type=target mount tells Docker to mount the
# current directory read-only in the WORKDIR. The type=cache mount tells Docker
# to cache the Go modules cache across builds.
RUN --mount=target=. \
    --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /function .

# Produce the Function image. We use a very lightweight 'distroless' image that
# does not include any of the build tools used in previous stages.
FROM gcr.io/distroless/base-debian11 AS image
WORKDIR /
COPY --from=build /function /function
COPY --from=package-stage /package.yaml /package.yaml
EXPOSE 9443
USER nonroot:nonroot
ENTRYPOINT ["/function"]