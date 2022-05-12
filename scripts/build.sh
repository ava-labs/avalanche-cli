#!/bin/bash

# Run with ./scripts/build.sh <optional_version>

if ! [[ "$0" =~ scripts/build.sh ]]; then
  echo "must be run from repository root"
  exit 1
fi

if [ $# -eq 0 ] ; then
    VERSION=`cat VERSION`
else
    VERSION=$1
fi

go build -v -ldflags="-X 'github.com/ava-labs/avalanche-cli/cmd.Version=$VERSION'" -o avalanche