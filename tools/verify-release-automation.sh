#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

VERSION_FILE="${REPO_ROOT}/backend/cmd/server/VERSION"
CHANGELOG_FILE="${REPO_ROOT}/CHANGELOG.md"
DEPLOY_README="${REPO_ROOT}/deploy/README.md"
DOCKER_README="${REPO_ROOT}/deploy/DOCKER.md"

required_workflows=(
  ".github/workflows/check-version-sync.yml"
  ".github/workflows/create-release.yml"
  ".github/workflows/publish-release-images.yml"
)

fail() {
  echo "ERROR: $*" >&2
  exit 1
}

pass() {
  echo "PASS: $*"
}

require_file() {
  local file="$1"
  [[ -f "${file}" ]] || fail "missing file: ${file#${REPO_ROOT}/}"
}

require_pattern() {
  local file="$1"
  local pattern="$2"
  local description="$3"

  if rg -q --fixed-strings "${pattern}" "${file}"; then
    pass "${description}"
    return 0
  fi

  fail "${description} not found in ${file#${REPO_ROOT}/}"
}

require_file "${VERSION_FILE}"
require_file "${CHANGELOG_FILE}"
require_file "${DEPLOY_README}"
require_file "${DOCKER_README}"

version="$(tr -d '[:space:]' < "${VERSION_FILE}")"
[[ -n "${version}" ]] || fail "backend/cmd/server/VERSION is empty"
[[ "${version}" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]] || \
  fail "backend/cmd/server/VERSION is not a semver-compatible version: ${version}"
pass "version file is semver-compatible (${version})"

for workflow in "${required_workflows[@]}"; do
  require_file "${REPO_ROOT}/${workflow}"
  pass "workflow exists (${workflow})"
done

require_pattern "${REPO_ROOT}/.github/workflows/check-version-sync.yml" "backend/cmd/server/VERSION" \
  "check-version-sync reads VERSION file"
require_pattern "${REPO_ROOT}/.github/workflows/check-version-sync.yml" "CHANGELOG.md" \
  "check-version-sync validates changelog"

require_pattern "${REPO_ROOT}/.github/workflows/create-release.yml" "git tag -a" \
  "create-release creates annotated tags"
require_pattern "${REPO_ROOT}/.github/workflows/create-release.yml" "backend/cmd/server/VERSION" \
  "create-release reads VERSION file"

require_pattern "${REPO_ROOT}/.github/workflows/publish-release-images.yml" "docker/build-push-action" \
  "publish-release-images builds Docker images"
require_pattern "${REPO_ROOT}/.github/workflows/publish-release-images.yml" "getLatestRelease" \
  "publish-release-images resolves latest GitHub release"

require_pattern "${DEPLOY_README}" "backend/cmd/server/VERSION" \
  "deploy README documents version source"
require_pattern "${DEPLOY_README}" "publish-release-images" \
  "deploy README documents image publish workflow"
require_pattern "${DOCKER_README}" "GitHub Container Registry" \
  "docker README documents GHCR"
require_pattern "${DOCKER_README}" "create-release.yml" \
  "docker README documents release workflow"

echo "All release automation checks passed."
