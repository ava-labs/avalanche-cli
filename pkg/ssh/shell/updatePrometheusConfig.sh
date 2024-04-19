#!/usr/bin/env bash
#name:TASK [sync new prometheus config]
sudo mkdir -p /etc/prometheus/
sudo cp -f /tmp/prometheus.yml /etc/prometheus/prometheus.yml
