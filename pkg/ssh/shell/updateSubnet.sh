#!/usr/bin/env bash

#name: stop node - stop avalanchego
sudo systemctl stop avalanchego
#name: import subnet
/home/ubuntu/bin/avalanche subnet import file {{ subnetExportFileName }} --force
#name: avalanche join subnet
/home/ubuntu/bin/avalanche subnet join {{ subnetName }} --fuji --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write
#name: restart node - start avalanchego
shell: sudo systemctl start avalanchego
