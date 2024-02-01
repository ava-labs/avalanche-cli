#!/usr/bin/env bash
set -e
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
#name:TASK [stop node - stop avalanchego]
sudo pkill avalanchego || echo "avalanchego not running"
#name:TASK [upgrade avalanchego version]
./avalanchego-installer.sh --version {{ .AvalancheGoVersion }}
#name:TASK [start node - start avalanchego] 
nohup /home/ubuntu/avalanche-node/avalanchego --config-file=/home/ubuntu/.avalanchego/configs/node.json </dev/null &>/dev/null &
sleep 2
