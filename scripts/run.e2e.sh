#!/usr/bin/env bash

set -e

label_filter="!ledger"

if [ "$1" = "--ledger" ]
then
    label_filter="ledger"
fi

if [ "$1" = "--ledger-sim" ]
then
    label_filter="ledger"
    export LEDGER_SIM="true"
fi

description_filter=""
if [ "$1" = "--filter" ]
then
    pat=$(echo $2 | tr ' ' '.')
    description_filter="--ginkgo.focus=$pat"
fi

export RUN_E2E="true"

if [ ! -d "tests/e2e/hardhat/node_modules" ]
then
    pushd tests/e2e/hardhat
    yarn
    popd
fi

if [ ! -d "tests/e2e/ledgerSim/node_modules" ]
then
    pushd tests/e2e/ledgerSim
    yarn
    cp node_modules/@zondax/zemu/dist/src/grpc/zemu.proto node_modules/@zondax/zemu/dist/grpc/zemu.proto
    popd
fi

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.1.3

extra_build_args=""
if [ "${LEDGER_SIM:-}" == true ]
then
	extra_build_args="-tags ledger_zemu"
fi

ACK_GINKGO_RC=true ginkgo build $extra_build_args ./tests/e2e

./tests/e2e/e2e.test --ginkgo.v --ginkgo.label-filter=$label_filter $description_filter

EXIT_CODE=$?

if [[ ${EXIT_CODE} -gt 0 ]]; then
  echo "FAILURE with exit code ${EXIT_CODE}"
  exit ${EXIT_CODE}
else
  echo "ALL SUCCESS!"
fi
