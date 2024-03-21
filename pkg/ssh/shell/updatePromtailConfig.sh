#!/usr/bin/env bash
#name:TASK [sync new promtail config]
sudo cp -f /tmp/promtail.yml /etc/promtail/config.yml
#name:TASK [restart prometail service]
sudo systemctl restart promtail
sudo chmod g+x /home/ubuntu/.avalanchego/logs || true
