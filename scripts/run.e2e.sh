#!/usr/bin/env bash

set -e

if [ "$1" = "--local" ]
then
    label_filter="local_machine"
else
    label_filter="!local_machine"
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

# Set the CGO flags to use the portable version of BLST
#
# We use "export" here instead of just setting a bash variable because we need
# to pass this flag to all child processes spawned by the shell.
export CGO_CFLAGS="-O -D__BLST_PORTABLE__"

go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.1.3
ACK_GINKGO_RC=true ginkgo build ./tests/e2e

./tests/e2e/e2e.test --ginkgo.v --ginkgo.label-filter=$label_filter $description_filter

EXIT_CODE=$?

if [[ ${EXIT_CODE} -gt 0 ]]; then
  echo "FAILURE with exit code ${EXIT_CODE}"
  exit ${EXIT_CODE}
else
  echo "ALL SUCCESS!"
fi
