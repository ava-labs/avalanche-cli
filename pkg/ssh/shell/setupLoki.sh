#!/usr/bin/env bash
#!/usr/bin/env bash
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [install loki]
{{ if .GrafanaPkg}}
curl -s https://apt.grafana.com/gpg.key | sudo apt-key add -
echo "deb https://apt.grafana.com stable main" | sudo tee /etc/apt/sources.list.d/grafana.list
sudo apt-get -y update
{{ end}}
sudo apt-get -y -o DPkg::Lock::Timeout=120 install loki promtail
sudo mkdir -p /var/lib/loki && sudo chown -R loki /var/lib/loki
echo "Provisioning datasource..."
{
    echo "apiVersion: 1"
    echo ""
    echo "datasources:"
    echo "  - name: Loki"
    echo "    type: loki"
    echo "    access: proxy"
    echo "    orgId: 1"
    echo "    url: http://localhost:23101"
    echo "    editable: false"
    echo "    jsonData:"
    echo "         timeout: 60"
    echo "         maxLines: 1000"
} >loki.yaml
sudo cp loki.yaml /etc/grafana/provisioning/datasources/
sudo systemctl restart grafana-server
