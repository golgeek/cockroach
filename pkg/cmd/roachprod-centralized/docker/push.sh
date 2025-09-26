#!/bin/bash

# Copyright 2021 The Cockroach Authors.
#
# Use of this software is governed by the CockroachDB Software License
# included in the /LICENSE file.

set -euo pipefail

# root is the absolute path to the root directory of the repository.
root="$(cd ../../../../ &> /dev/null && pwd)"
echo "Using root directory: $root"

SHA=$(git rev-parse --short HEAD)
IMAGE=us-central1-docker.pkg.dev/cockroach-testeng-infra/roachprod/roachprod-centralized

echo "Building and pushing image ${IMAGE}:${SHA}"

# Clean up any previous images or manifests
podman image rm -i "${IMAGE}:${SHA}"
podman manifest rm -i "${IMAGE}:${SHA}"

# Create a manifest for the image
podman manifest create "${IMAGE}:${SHA}"

# Build the image for amd64 and arm64 platforms
podman buildx build \
  --platform linux/amd64 \
  --manifest "${IMAGE}:${SHA}" \
  --layers \
  --format docker \
  --cache-from "${IMAGE}-cache" \
  --cache-to "${IMAGE}-cache" \
  --file Dockerfile \
  $root

# Push the image to the registry
podman manifest push "${IMAGE}:${SHA}"

echo "Image pushed to ${IMAGE}:${SHA}"