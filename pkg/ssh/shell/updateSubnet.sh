#!/usr/bin/env bash

echo {{ .Log }}TASK [stop node - stop avalanchego]
sudo systemctl stop avalanchego
echo {{ .Log }}TASK [import subnet]
/home/ubuntu/bin/avalanche subnet import file {{ .SubnetExportFileName }} --force
echo {{ .Log }}TASK [avalanche join subnet]
/home/ubuntu/bin/avalanche subnet join {{ .SubnetName }} --fuji --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write
echo {{ .Log }}TASK [restart node - start avalanchego]
sudo systemctl start avalanchego
