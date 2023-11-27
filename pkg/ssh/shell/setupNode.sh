#!/usr/bin/env bash
#name:TASK [update apt data and install dependencies] 
DEBIAN_FRONTEND=noninteractive sudo apt-get -y update
DEBIAN_FRONTEND=noninteractive sudo apt-get -y install wget curl git
#name:TASK [create .avalanche-cli .avalanchego dirs]
mkdir -p .avalanche-cli .avalanchego/staking
#name:TASK [get avalanche go script]
wget -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-docs/master/scripts/avalanchego-installer.sh
#name:TASK [modify permissions]
chmod 755 avalanchego-installer.sh
#name:TASK [call avalanche go install script]
./avalanchego-installer.sh --ip static --rpc private --state-sync on --fuji --version {{ .AvalancheGoVersion }}
#name:TASK [get avalanche cli install script]
wget -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]
./install.sh -n
{{if .IsDevNet}}
#name:TASK [stop avalanchego in case of devnet]
sudo systemctl stop avalanchego
{{end}}
