#!/usr/bin/env bash
# Copyright (C) 2023, Ava Labs, Inc. All rights reserved.
# See the file LICENSE for licensing terms.

set -o errexit
set -o nounset
set -o pipefail

CLI_PATH=$(
    cd "$(dirname "${BASH_SOURCE[0]}")"
    cd .. && pwd
)

GOLANGCI_LINT_VERSION=v1.64.5

# avoid calling go install unless it is needed: makes the script able to be used offline
do_install=true
which golangci-lint > /dev/null 2>&1
if [ $? = 0 ]
then
	golangci-lint --version | grep $GOLANGCI_LINT_VERSION > /dev/null 2>&1
	[ $? = 0 ] && do_install=false
fi
if [ $do_install = true ]
then
	go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@${GOLANGCI_LINT_VERSION}
fi

golangci-lint run --config=$CLI_PATH/.golangci.yml ./... --timeout 5m
