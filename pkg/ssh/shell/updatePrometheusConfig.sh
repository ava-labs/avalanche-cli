#!/usr/bin/env bash
#name:TASK [delete existing monitored hosts in prometheus config]
sudo sed -i.bak -e '30,38d' /etc/prometheus/prometheus.yml
#name:TASK [set up node_exporter]
./monitoring-separate-installer.sh --6 "{{ .AvalancheGoPorts }}" "{{ .MachinePorts }}"