#!/usr/bin/env bash
export PATH=$PATH:~/go/bin
mkdir -p ~/.avalanche-cli
{{ if .IsE2E }}
echo "E2E detected"
export DEBIAN_FRONTEND=noninteractive
apt-get update -y && sudo apt-get install busybox-static -y
{{ end }}
{{ if .IsCI }}
{{ end }}
busybox wget -q -nd https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]
./install.sh -n {{ .CLIVersion }}
