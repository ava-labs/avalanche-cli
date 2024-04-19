#!/usr/bin/env bash
#name:TASK [sync new loki config]
sudo mkdir -p /etc/loki
sudo cp -f /tmp/loki.yml /etc/loki/config.yml
