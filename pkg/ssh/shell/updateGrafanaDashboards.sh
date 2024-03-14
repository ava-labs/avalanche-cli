#!/usr/bin/env bash
#name:TASK [sync grafana dashboards]
sudo cp -f /home/ubuntu/dashboards/* /etc/grafana/dashboards/
#name:TASK [restart prometheus service]
sudo systemctl restart grafana-server
