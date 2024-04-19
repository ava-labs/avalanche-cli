#!/usr/bin/env bash
#name:TASK [sync new loki config]
mkdir -p /etc/loki
sudo cp -f /tmp/loki.yml /etc/loki/config.yml
