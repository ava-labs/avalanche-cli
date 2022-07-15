#!/bin/bash

if ! [[ "$0" =~ scripts/regenerate_mocks.sh ]]; then
  echo "must be run from repository root"
  exit 1
fi

go install github.com/vektra/mockery/v2@latest

mockery -r --output ./internal/mocks --case camel --name BinaryChecker
mockery -r --output ./internal/mocks --case camel --name PluginBinaryDownloader
mockery -r --output ./internal/mocks --case camel --name ProcessChecker
mockery -r --output ./internal/mocks --case camel --name Prompter
