#!/usr/bin/env bash
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [download monitoring script]
wget -q -nd -m https://raw.githubusercontent.com/ava-labs/avalanche-monitoring/main/grafana/monitoring-installer.sh
#name:TASK [modify permission for monitoring script]
chmod 755 monitoring-installer.sh
#name:TASK [set up Prometheus]
while ! sudo systemctl status prometheus >/dev/null 2>&1; do
   ./monitoring-installer.sh --1
    if [ $? -ne 0 ]; then
        echo "Failed to install Prometheus. Retrying in 10 seconds..."
        sleep 10
    fi
done
#name:TASK [install Grafana]
while ! sudo systemctl status grafana-server >/dev/null 2>&1; do
    ./monitoring-installer.sh --2
    if [ $? -ne 0 ]; then
        echo "Failed to install Grafana. Retrying in 10 seconds..."
        sleep 10
    fi
done
#name:TASK [set up node_exporter]
while ! sudo systemctl status node_exporter >/dev/null 2>&1; do
    ./monitoring-installer.sh --3
    if [ $? -ne 0 ]; then
        echo "Failed to install Node_Exporter. Retrying in 10 seconds..."
        sleep 10
    fi
done
#name:TASK [set up dashboards]
./monitoring-installer.sh --4
