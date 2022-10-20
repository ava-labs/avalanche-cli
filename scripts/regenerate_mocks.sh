#!/bin/bash


# NOTE: This script can only generate mocks from interfaces inside this repository.
# For interfaces in other repositories (e.g. avalanchego), the mocks should 
# still be created manually for the time being.
# 
# This currently affects the following mocks:
# * InfoClient
# * PClient

if ! [[ "$0" =~ scripts/regenerate_mocks.sh ]]; then
  echo "must be run from repository root"
  exit 1
fi

go install github.com/vektra/mockery/v2@latest

mockery -r --output ./internal/mocks --name BinaryChecker --filename binaryChecker.go
mockery -r --output ./internal/mocks --name PluginBinaryDownloader --filename pluginBinaryDownloader.go
mockery -r --output ./internal/mocks --name ProcessChecker --filename processChecker.go
mockery -r --output ./internal/mocks --name Prompter --filename prompter.go
mockery -r --output ./internal/mocks --name Installer --filename installer.go
mockery -r --output ./internal/mocks --name Publisher --filename publisher.go 

echo ""
echo "Created mocks for interfaces in this repository only. Please create other mocks manually."
