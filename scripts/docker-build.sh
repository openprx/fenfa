#!/bin/sh
set -eu

# Multi-arch build helper for fenfa
# Env:
#   IMAGE       - image name (default: fenfa/fenfa)
#   TAG         - image tag (default: latest)
#   PLATFORMS   - platforms to build (default: linux/amd64,linux/arm64)
#   PUSH        - set to "true" to push to registry, otherwise --load local (default: false)

IMAGE=${IMAGE:-fenfa/fenfa}
TAG=${TAG:-latest}
PLATFORMS=${PLATFORMS:-linux/amd64,linux/arm64}
PUSH=${PUSH:-false}

# Ensure buildx is available
if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required. Install Docker Desktop or docker buildx plugin." >&2
  exit 1
fi

if [ "$PUSH" = "true" ]; then
  echo "Building and pushing $IMAGE:$TAG for platforms: $PLATFORMS"
  docker buildx build \
    --platform "$PLATFORMS" \
    -t "$IMAGE:$TAG" \
    --push \
    .
else
  # Local load for current arch only
  ARCH=$(uname -m)
  case "$ARCH" in
    x86_64) PLATFORM=linux/amd64 ;;
    aarch64|arm64) PLATFORM=linux/arm64 ;;
    *) PLATFORM=linux/amd64 ;;
  esac
  echo "Building locally for $PLATFORM and loading into Docker engine: $IMAGE:$TAG"
  docker buildx build \
    --platform "$PLATFORM" \
    -t "$IMAGE:$TAG" \
    --load \
    .
fi

