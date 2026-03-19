#!/usr/bin/env bash
set -euo pipefail

# Usage: ./scripts/release.sh <version>
# Example: ./scripts/release.sh 0.1.2

VERSION="${1:-}"

if [ -z "$VERSION" ]; then
  echo "Usage: $0 <version>"
  echo "Example: $0 0.1.2"
  exit 1
fi

# Strip leading 'v' if provided
VERSION="${VERSION#v}"
TAG="v${VERSION}"

# Validate semver format
if ! echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
  echo "Error: version must be semver format (e.g. 0.1.2)"
  exit 1
fi

# Check working tree is clean
if [ -n "$(git status --porcelain)" ]; then
  echo "Error: working tree is dirty. Commit or stash changes first."
  git status --short
  exit 1
fi

# Check required env vars
if [ -z "${GITHUB_TOKEN:-}" ]; then
  echo "Error: GITHUB_TOKEN is not set"
  exit 1
fi

if [ -z "${HOMEBREW_TAP_GITHUB_TOKEN:-}" ]; then
  echo "Warning: HOMEBREW_TAP_GITHUB_TOKEN not set — homebrew tap will not be updated"
fi

echo "Releasing ${TAG}..."

# Delete existing tag if present (local + remote)
if git rev-parse "$TAG" >/dev/null 2>&1; then
  echo "Tag ${TAG} already exists locally — deleting and recreating"
  git tag -d "$TAG"
  git push origin ":refs/tags/${TAG}" 2>/dev/null || true
fi

# Create and push tag
git tag "$TAG"
git push origin "$TAG"

echo "Tag ${TAG} pushed. Running goreleaser..."

goreleaser release --clean

echo "Done. Release ${TAG} complete."
