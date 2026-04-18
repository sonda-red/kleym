#!/usr/bin/env bash
# Show commits since the last release tag and suggest a version bump
# based on Conventional Commit prefixes.
set -euo pipefail

LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || true)

if [[ -z "$LAST_TAG" ]]; then
  echo "No previous tags found."
  RANGE="HEAD"
else
  echo "Last release: $LAST_TAG ($(git log -1 --format='%ai' "$LAST_TAG"))"
  RANGE="$LAST_TAG..HEAD"
fi

echo ""

SUBJECTS=$(git log "$RANGE" --format='%s' --)

if [[ -z "$SUBJECTS" ]]; then
  echo "No new commits since $LAST_TAG."
  exit 0
fi

echo "Commits since ${LAST_TAG:-inception}:"
git log --oneline "$RANGE" --
echo ""

# Detect version-relevant signals from commit subjects
HAS_BREAKING=false
HAS_FEAT=false

if echo "$SUBJECTS" | grep -qiE '^[a-z]+(\(.+\))?!:|BREAKING CHANGE'; then
  HAS_BREAKING=true
fi

if echo "$SUBJECTS" | grep -qiE '^feat(\(.+\))?[!]?:'; then
  HAS_FEAT=true
fi

if [[ -z "$LAST_TAG" ]]; then
  echo "Suggested first release: v0.1.0"
  exit 0
fi

MAJOR=$(echo "$LAST_TAG" | sed 's/^v//' | cut -d. -f1)
MINOR=$(echo "$LAST_TAG" | sed 's/^v//' | cut -d. -f2)
PATCH=$(echo "$LAST_TAG" | sed 's/^v//' | cut -d. -f3)

if $HAS_BREAKING; then
  echo "Breaking changes detected."
  echo "Suggested bump: major -> v$((MAJOR + 1)).0.0"
elif $HAS_FEAT; then
  echo "New features detected."
  echo "Suggested bump: minor -> v${MAJOR}.$((MINOR + 1)).0"
else
  echo "Fixes and maintenance only."
  echo "Suggested bump: patch -> v${MAJOR}.${MINOR}.$((PATCH + 1))"
fi
