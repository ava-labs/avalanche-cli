#!/usr/bin/env bash
#name:TASK [set up node_exporter] 
./monitoring-separate-installer.sh --3 "{{ .AvalancheGoPorts }}" "{{ .MachinePorts }}"
