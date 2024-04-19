#!/usr/bin/env bash
#!/usr/bin/env bash
{{if .IsE2E }}
#name:TASK [disable systemctl]
sudo cp -vf /usr/bin/true /usr/local/sbin/systemctl
{{end}}
#name:TASK [install loki]
sudo mkdir -p /var/lib/loki /etc/grafana/provisioning/datasources
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
