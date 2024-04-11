#!/usr/bin/env bash
#!/usr/bin/env bash
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [add repository]
curl -s https://apt.grafana.com/gpg.key | sudo apt-key add -
echo "deb https://apt.grafana.com stable main" | sudo tee /etc/apt/sources.list.d/grafana.list
sudo apt-get -y -o DPkg::Lock::Timeout=120 update
#name:TASK [install promtail]
sudo apt-get -y -o DPkg::Lock::Timeout=120 install promtail
sudo usermod -a -G ubuntu promtail
sudo chmod g+x /home/ubuntu/.avalanchego/logs || true
