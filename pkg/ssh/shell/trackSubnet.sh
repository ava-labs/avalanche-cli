#!/ust/bin/env bash
#name:{{ .Log }}TASK [import subnet]
/home/ubuntu/bin/avalanche subnet import file {{ .SubnetExportFileName }} --force
#name:{{ .Log }}TASK [avalanche join subnet]
/home/ubuntu/bin/avalanche subnet join {{ .SubnetName }} --fuji --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write
#name:{{ .Log }}TASK [restart node - restart avalanchego]
sudo systemctl restart avalanchego
