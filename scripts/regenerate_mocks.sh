#!/usr/bin/env bash


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

# SDK
go install go.uber.org/mock/mockgen@v0.5.1
subnet_evm_version=$(grep subnet-evm go.mod | awk '{print $NF}')
mockgen -source=$(go env GOPATH)/pkg/mod/github.com/ava-labs/subnet-evm@$subnet_evm_version/ethclient/ethclient.go -destination=sdk/mocks/ethclient/mock_ethclient.go Client

# CLI
go install github.com/vektra/mockery/v2@v2.43.2
mockery -r --output ./internal/mocks --name BinaryChecker --filename binary_checker.go
mockery -r --output ./internal/mocks --name PluginBinaryDownloader --filename plugin_binary_downloader.go
mockery -r --output ./internal/mocks --name Prompter --filename prompter.go
mockery -r --output ./internal/mocks --name Installer --filename installer.go
mockery -r --output ./internal/mocks --name Publisher --filename publisher.go
mockery -r --output ./internal/mocks --name Downloader --filename downloader.go

echo ""
echo "Created mocks for interfaces in this repository only. Please create other mocks manually."
