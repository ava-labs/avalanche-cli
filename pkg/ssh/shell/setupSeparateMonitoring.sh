#!/usr/bin/env bash
#name:TASK [modify permission for monitoring script]
chmod 755 monitoring-separate-installer.sh
#name:TASK [set up Prometheus]
./monitoring-separate-installer.sh --1
#name:TASK [install Grafana]
./monitoring-separate-installer.sh --2
#name:TASK [set up node_exporter]
./monitoring-separate-installer.sh --3 "{{ .AvalancheGoPorts }}" "{{ .MachinePorts }}"
#name:TASK [set up dashboards]
./monitoring-separate-installer.sh --4