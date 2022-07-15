#!/bin/bash

if ! [[ "$0" =~ scripts/regenerate_mocks.sh ]]; then
  echo "must be run from repository root"
  exit 1
fi

go install github.com/vektra/mockery/v2@latest

mockery --all --output ./internal/mocks
