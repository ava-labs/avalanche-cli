#!/usr/bin/bash

set -e

export RUN_E2E="true"

go install -v github.com/onsi/ginkgo/v2/ginkgo@v2.1.3
ACK_GINKGO_RC=true ginkgo build ./tests/e2e

./tests/e2e/e2e.test --ginkgo.v

EXIT_CODE=$?

if [[ ${EXIT_CODE} -gt 0 ]]; then
  echo "FAILURE with exit code ${EXIT_CODE}"
  exit ${EXIT_CODE}
else
  echo "ALL SUCCESS!"
fi
