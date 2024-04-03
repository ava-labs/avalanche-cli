#!/usr/bin/env bash
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
./monitoring-installer.sh --1
#name:TASK [install Grafana]
./monitoring-installer.sh --2
#name:TASK [set up node_exporter]
./monitoring-installer.sh --3
#name:TASK [set up dashboardst]
./monitoring-installer.sh --4
