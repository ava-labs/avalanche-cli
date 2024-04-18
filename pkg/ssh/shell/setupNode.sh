#!/usr/bin/env bash
export PATH=$PATH:~/go/bin
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [create .avalanche-cli .avalanchego dirs]
mkdir -p .avalanche-cli .avalanchego/staking
#name:TASK [call avalanche go install script]
#./avalanchego-installer.sh --ip static --rpc private --state-sync on --fuji --version {{ .AvalancheGoVersion }}
#name:TASK [get avalanche cli install script]
busybox wget -q -nd https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]
./install.sh -n {{ .CLIVersion }}
