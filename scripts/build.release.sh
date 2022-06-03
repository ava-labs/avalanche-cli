#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

if ! [[ "$0" =~ scripts/build.release.sh ]]; then
  echo "must be run from repository root"
  exit 255
fi

# https://goreleaser.com/install/
#TODO: latest requires go1.18
#go install -v github.com/goreleaser/goreleaser@latest
go install -v github.com/goreleaser/goreleaser@v1.6.3

# e.g.,
# git tag 1.0.0
goreleaser release \
--config .goreleaser.yml \
--skip-announce \
--skip-publish

# to test without git tags
# goreleaser release --config .goreleaser.yml --rm-dist --skip-announce --skip-publish --snapshot
