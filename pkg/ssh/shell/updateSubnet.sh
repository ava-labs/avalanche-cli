#!/usr/bin/env bash
#name:TASK [stop node - stop avalanchego]
sudo systemctl stop avalanchego
#name:TASK [import subnet]
/home/ubuntu/bin/avalanche subnet import file {{ .SubnetExportFileName }} --force
#name:TASK [avalanche join subnet]
/home/ubuntu/bin/avalanche subnet join {{ .SubnetName }} --fuji --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write
#name:TASK [restart node - start avalanchego]
sudo systemctl start avalanchego
