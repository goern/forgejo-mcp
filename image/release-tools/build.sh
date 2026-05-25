#!/usr/bin/env bash
# Single-shot local image build via podman. No push.
# Mirrors the PR pipeline so contributors can reproduce locally.
# Requires: podman, git
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

IMAGE_NAME="${IMAGE_NAME:-localhost/release-tools}"
IMAGE_TAG="${IMAGE_TAG:-dev}"

echo "Building ${IMAGE_NAME}:${IMAGE_TAG} from ${SCRIPT_DIR}"

# Build context is SCRIPT_DIR (image/release-tools/) so COPY paths in the
# Containerfile resolve correctly (e.g. `COPY npm/package.json` → image/release-tools/npm/).
# .dockerignore at image/release-tools/.dockerignore is the active ignore file.
podman build \
    --file "${SCRIPT_DIR}/Containerfile" \
    --tag "${IMAGE_NAME}:${IMAGE_TAG}" \
    --build-arg HI_GO_TAG=latest-builder \
    --build-arg HI_GO_DIGEST=sha256:d8c8b702b8a54150e8fdca86753f581d98c551ab8a3fd429886d4ddd4e949894 \
    --build-arg SYFT_VERSION=v1.44.0 \
    --build-arg GORELEASER_VERSION=v2.16.0 \
    --build-arg COSIGN_VERSION=v3.0.6 \
    --build-arg GOVULNCHECK_VERSION=latest \
    --build-arg MCPB_VERSION=2.1.2 \
    "${SCRIPT_DIR}"

echo "Build complete: ${IMAGE_NAME}:${IMAGE_TAG}"
echo "Run verify.sh to assert tool versions:"
echo "  RELEASE_TOOLS_IMAGE=${IMAGE_NAME}:${IMAGE_TAG} bash ${SCRIPT_DIR}/verify.sh"
