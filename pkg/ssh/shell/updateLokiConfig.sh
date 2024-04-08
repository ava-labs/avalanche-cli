#!/usr/bin/env bash
#name:TASK [sync new loki config]
sudo cp -f /tmp/loki.yml /etc/loki/config.yml
#name:TASK [restart loki service]
sudo systemctl restart loki
sleep 1 && curl -s http://localhost:23101/ready || true
