#!/usr/bin/env bash
export PATH=$PATH:~/go/bin
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [create .avalanche-cli .avalanchego dirs]
mkdir -p .avalanche-cli .avalanchego/staking
foundIP="$(dig +short myip.opendns.com @resolver1.opendns.com)"
pushd .avalanchego
rm -f node.json
echo "{" >>node.json
echo "  \"http-host\": \"0.0.0.0\",">>node.json
echo "  \"api-admin-enabled\": true,">>node.json
echo "  \"network-id\": \"fuji\",">>node.json
echo "  \"public-ip\": \"$foundIP\"">>node.json
echo "}" >>node.json
mkdir -p $HOME/.avalanchego/configs
cp -f node.json $HOME/.avalanchego/configs/node.json

rm -f config.json
echo "{" >>config.json
echo "  \"state-sync-enabled\": true">>config.json
echo "}" >>config.json
mkdir -p $HOME/.avalanchego/configs/chains/C
cp -f config.json $HOME/.avalanchego/configs/chains/C/config.json
popd
#name:TASK [call avalanche go install script]
#./avalanchego-installer.sh --ip static --rpc private --state-sync on --fuji --version {{ .AvalancheGoVersion }}
#name:TASK [get avalanche cli install script]
busybox wget -q -nd https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]
./install.sh -n {{ .CLIVersion }}
