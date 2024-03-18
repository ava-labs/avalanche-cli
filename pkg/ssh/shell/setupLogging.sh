#!/usr/bin/env bash
#!/usr/bin/env bash
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [install loki]
sudo apt-get -y -o DPkg::Lock::Timeout=120 install loki promtail
sudo mkdir -p /var/lib/loki && sudo chown -R loki /var/lib/loki
