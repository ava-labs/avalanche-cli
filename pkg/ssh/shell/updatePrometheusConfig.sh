#!/usr/bin/env bash
#name:TASK [sync new prometheus config]
sudo cp -f /tmp/prometheus.yml /etc/prometheus/prometheus.yml
#name:TASK [restart prometheus service]
sudo systemctl restart prometheus
