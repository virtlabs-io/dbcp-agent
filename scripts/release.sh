#!/bin/bash
# scripts/release.sh

VERSION=$1
if [ -z "$VERSION" ]; then
  echo "Usage: ./scripts/release.sh v1.0.0"
  exit 1
fi

echo "Tagging release $VERSION"
git tag "$VERSION"
git push origin "$VERSION"

echo "Generating changelog"
git log --pretty=format:"* %s (%h)" $(git describe --tags --abbrev=0)..HEAD > CHANGELOG.md
cat CHANGELOG.md

echo "Building binaries"
make clean
make build
mkdir -p release
cp dbcp-agent release/dbcp-agent-$VERSION-linux-amd64
