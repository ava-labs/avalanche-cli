#!/usr/bin/env bash
set -e
export PATH=$PATH:~/go/bin:~/.cargo/bin
/home/ubuntu/bin/avalanche subnet import file {{ .SubnetExportFileName }} --force
sudo systemctl stop avalanchego
/home/ubuntu/bin/avalanche subnet join {{ .SubnetName }} {{ .NetworkFlag }} --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write
sudo systemctl start avalanchego
