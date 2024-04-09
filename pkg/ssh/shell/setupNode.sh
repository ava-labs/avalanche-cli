#!/usr/bin/env bash
set -e
export PATH=$PATH:~/go/bin
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [update apt data and install dependencies]
export DEBIAN_FRONTEND=noninteractive
until sudo apt-get -y install -o DPkg::Lock::Timeout=120 wget curl git; do sleep 10 && echo "Try again"; done
#name:TASK [create .avalanche-cli .avalanchego dirs]
mkdir -p .avalanche-cli .avalanchego/staking
#name:TASK [get avalanche go script]
wget -q -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-docs/master/scripts/avalanchego-installer.sh
#name:TASK [modify permissions]
chmod 755 avalanchego-installer.sh
#name:TASK [call avalanche go install script]
./avalanchego-installer.sh --ip static --rpc private --state-sync on --fuji --version {{ .AvalancheGoVersion }}
#name:TASK [get avalanche cli install script]
wget -q -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]
./install.sh -n {{ .CLIVersion }}
{{if .IsDevNet}}
#name:TASK [stop avalanchego in case of devnet]
{{if .IsE2E }}
sudo pkill avalanchego || echo "avalanchego not running"
{{ else }}
sudo systemctl stop avalanchego
{{end}}
{{end}}
