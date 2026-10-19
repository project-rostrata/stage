#!/usr/bin/env bash
# Build a release and commit it: ./release.sh v1.0.0
# The binary goes in dist/, the one binary kept in git, so a machine can
# install stage with curl and no checkout. The commit is tagged with the version.
set -euo pipefail
cd "$(dirname "$0")"
version="${1:?usage: ./release.sh vX.Y.Z}"
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo "version must look like v1.2.3" >&2; exit 1; }
[[ -z "$(git status --porcelain)" ]] || { echo "commit or stash your changes first" >&2; exit 1; }
git rev-parse -q --verify "refs/tags/$version" >/dev/null && { echo "$version is already tagged" >&2; exit 1; }

./test.sh
scripts/fetch-go.sh
export PATH="$PWD/.toolchain/go/bin:$PATH" GOTOOLCHAIN=local
mkdir -p dist
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
	-ldflags "-s -w -X main.version=$version" -o dist/stage-linux-amd64 .
(cd dist && sha256sum stage-linux-amd64 > SHA256SUMS)

git add dist
git commit -q -m "Release $version"
git tag -a "$version" -m "stage $version"
echo "Released $version. Push it with: git push origin main $version"
