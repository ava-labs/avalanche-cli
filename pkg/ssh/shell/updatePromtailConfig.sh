#!/usr/bin/env bash
#name:TASK [sync new promtail config]
sudo mkdir -p /etc/promtail/
sudo cp -f /tmp/promtail.yml /etc/promtail/config.yml
