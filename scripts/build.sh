#!/bin/bash

# Run with ./scripts/build.sh <optional_version>
TELEMETRY_TOKEN=""
if ! [[ "$0" =~ scripts/build.sh ]]; then
  echo "must be run from repository root"
  exit 1
fi

if [ $# -eq 0 ] ; then
    VERSION=`cat VERSION`
else
    VERSION=$1
fi

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

extra_build_args=""
if [ "${LEDGER_SIM:-}" == true ]
then
	extra_build_args="-tags ledger_zemu"
fi

go build -v -ldflags="-X 'github.com/ava-labs/avalanche-cli/cmd.Version=$VERSION' -X github.com/ava-labs/avalanche-cli/pkg/utils.telemetryToken=$TELEMETRY_TOKEN" $extra_build_args -o bin/avalanche
