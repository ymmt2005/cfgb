#!/usr/bin/env bash
# Adapted from pbschema-lens/scripts/release-detect.sh.
# HEAD vs HEAD^ is sufficient because main uses squash merges.
set -euo pipefail

emit() {
  echo "$1=$2"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "$1=$2" >> "$GITHUB_OUTPUT"
  fi
}

source_version() {
  local version
  version="$(sed -nE 's/^var Version( string)?[[:space:]]*=[[:space:]]*"([^"[:cntrl:]]+)".*/\2/p')"
  if [[ -z "$version" ]]; then
    echo "Missing Version string in internal/version/version.go" >&2
    return 1
  fi
  printf '%s' "$version"
}

skip() {
  emit tag "$tag"
  emit release false
  emit verify false
}

tag="$(source_version < internal/version/version.go)"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "Skip: Version is not an exact release tag ($tag)"
  skip
  exit 0
fi

# Match the prerelease identifiers accepted by GoReleaser's parser before
# pushing a tag. Its NewVersion parser permits leading zeros in core versions.
if [[ "$tag" == *-* ]]; then
  prerelease="${tag#*-}"
  if [[ "$prerelease" == .* || "$prerelease" == *. || "$prerelease" == *..* ]]; then
    echo "Skip: empty prerelease identifier ($tag)"
    skip
    exit 0
  fi
  IFS='.' read -r -a identifiers <<< "$prerelease"
  for identifier in "${identifiers[@]}"; do
    if [[ "$identifier" =~ ^0[0-9]+$ ]]; then
      echo "Skip: numeric prerelease identifier starts with zero ($tag)"
      skip
      exit 0
    fi
  done
fi

if git rev-parse --verify --quiet HEAD^ >/dev/null; then
  prev="$(git show HEAD^:internal/version/version.go | source_version)"
  if [[ "$prev" == "$tag" ]]; then
    echo "Skip: Version unchanged ($tag)"
    skip
    exit 0
  fi
  echo "Version change $prev -> $tag"
else
  status=$?
  if [[ "$status" != 1 ]]; then
    echo "Cannot inspect parent commit" >&2
    exit "$status"
  fi
  echo "No parent commit; treating $tag as a version change."
fi

# Preserve the HTTP status: an authentication/network/API failure must not
# look like an absent release. Only 404 means this version has no release.
response_file="$(mktemp)"
trap 'rm -f "$response_file"' EXIT
status="$(curl --silent --show-error \
  --output "$response_file" --write-out '%{http_code}' \
  --header 'Accept: application/vnd.github+json' \
  --header "Authorization: Bearer ${GH_TOKEN:?GH_TOKEN is required}" \
  "https://api.github.com/repos/${GITHUB_REPOSITORY:-ymmt2005/cfgb}/releases/tags/$tag")"
case "$status" in
  404) ;;
  200)
    if [[ "$(jq -r '.draft' "$response_file")" != true ]]; then
      if [[ "$(jq -r '.immutable' "$response_file")" != true ]]; then
        echo "Existing release is not immutable" >&2
        exit 1
      fi
      echo "Skip: immutable release $tag already exists"
      emit tag "$tag"
      emit release false
      emit verify true
      exit 0
    fi
    ;;
  *)
    echo "Release lookup failed: HTTP $status" >&2
    exit 1
    ;;
esac

emit tag "$tag"
emit release true
emit verify true
