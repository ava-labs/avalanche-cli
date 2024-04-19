#!/usr/bin/env bash
#name:TASK [sync grafana dashboards]
sudo mkdir -p /etc/grafana/provisioning/dashboards/
sudo cp -f /home/ubuntu/dashboards/* /etc/grafana/provisioning/dashboards/
#name:TASK [restart prometheus service]
sudo systemctl restart grafana-server
