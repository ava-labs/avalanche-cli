#!/usr/bin/env bash

#name: import subnet
/home/ubuntu/bin/avalanche subnet import file {{ subnetExportFileName }} --force
#name: avalanche join subnet
/home/ubuntu/bin/avalanche subnet join {{ subnetName }} --fuji --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write
#name: restart node - restart avalanchego
sudo systemctl restart avalanchego
