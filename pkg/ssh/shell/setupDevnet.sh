#!/usr/bin/env bash
set -e
#name:TASK [stop node]
{{if .IsE2E}}
sudo pkill avalanchego || echo "avalanchego not running"
{{else}}
sudo systemctl daemon-reload
sudo systemctl stop avalanchego
{{end}}
#name:TASK [remove previous avalanchego db and logs]
rm -rf /home/ubuntu/.avalanchego/db/
rm -rf /home/ubuntu/.avalanchego/logs/
#name:TASK [start node]
{{if .IsE2E}}
nohup /home/ubuntu/avalanche-node/avalanchego --config-file=/home/ubuntu/.avalanchego/configs/node.json </dev/null &>/dev/null &
sleep 2
{{else}}
sudo systemctl start avalanchego
{{end}}
