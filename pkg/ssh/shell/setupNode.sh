#!/usr/bin/env bash

#name: get avalanche go script
wget -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-docs/master/scripts/avalanchego-installer.sh
#name: modify permissions
chmod 755 avalanchego-installer.sh
#name: call avalanche go install script
./avalanchego-installer.sh --ip static --rpc private --state-sync on --fuji --version {{ .AvalancheGoVersion }}
#name: get avalanche cli install script
wget -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name: modify permissions
chmod 755 install.sh
#name: run install script
./install.sh -n
#name: create .avalanche-cli dir
mkdir -p .avalanche-cli
