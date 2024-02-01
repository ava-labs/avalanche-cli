#!/usr/bin/env bash
set -e
#name:TASK [stop node - stop avalanchego]
{{if .IsE2E }}
sudo pkill avalanchego || echo "avalanchego not running"
{{ else }}
sudo systemctl stop avalanchego
{{end}}
#name:TASK [import subnet]
/home/ubuntu/bin/avalanche subnet import file {{ .SubnetExportFileName }} --force
#name:TASK [avalanche join subnet]
/home/ubuntu/bin/avalanche subnet join {{ .SubnetName }} --fuji --avalanchego-config /home/ubuntu/.avalanchego/configs/node.json --plugin-dir /home/ubuntu/.avalanchego/plugins --force-write
#name:TASK [restart node - start avalanchego]
{{if .IsE2E }}
nohup /home/ubuntu/avalanche-node/avalanchego --config-file=/home/ubuntu/.avalanchego/configs/node.json </dev/null &>/dev/null &
sleep 2
{{ else }}
sudo systemctl start avalanchego
{{end}}
