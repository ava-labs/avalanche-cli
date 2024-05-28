#!/usr/bin/env bash

set -e

description_filter=""
if [ "$1" = "--filter" ]
then
    pat=$(echo $2 | tr ' ' '.')
    description_filter="--ginkgo.focus=$pat"
fi

export RUN_E2E="true"
#github runner detected 
current_user=$(whoami)

# Check if the current user is 'runner'
if [ "$current_user" = "runner" ] && [ "$OSTYPE" == "linux-gnu"* ]; then
    echo "github action[runner]"
    sudo chown runner /var/run/docker.sock
    sudo chmod +rw /var/run/docker.sock
    sudo useradd -m -s /bin/bash -u 1000 ubuntu && sudo mkdir -p /home/ubuntu && sudo chown -R 1000:1000 /home/ubuntu || echo "failed to create ubuntu user"
    sudo mkdir -p /home/ubuntu/.avalanche-cli /home/ubuntu/.avalanchego 
    sudo chown -R 1000:1000 /home/ubuntu || echo "failed to change ownership of /home/ubuntu to ubuntu user"
    for i in $(seq 1 9) ; do
        sudo ifconfig lo:$i 192.168.223.10$i up
    done
    sudo docker system prune -f || echo "failed to cleanup docker"
fi

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

./tests/e2e/e2e.test --ginkgo.v $description_filter

EXIT_CODE=$?

if [[ ${EXIT_CODE} -gt 0 ]]; then
  echo "FAILURE with exit code ${EXIT_CODE}"
  exit ${EXIT_CODE}
else
  echo "ALL SUCCESS!"
fi
