#!/usr/bin/env bash
export PATH=$PATH:~/go/bin
mkdir -p ~/.avalanche-cli
{{ if .IsE2E }}
echo "E2E detected"
echo "CLI Version: {{ .CLIVersion }}"
export DEBIAN_FRONTEND=noninteractive
sudo apt-get update -y && sudo apt-get install busybox-static -y
sudo apt-get -y update && sudo apt-get -y dist-upgrade && sudo apt-get -y install ca-certificates curl gcc git golang-go=2:1.22~3longsleep1
sudo install -m 0755 -d /etc/apt/keyrings && sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && sudo chmod a+r /etc/apt/keyrings/docker.asc
echo \"deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable\" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get -y update && sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-compose
sudo usermod -aG docker ubuntu
{{ end }}
{{ if .IsE2E }}
echo "E2E skip install"
exit 0
{{ end }}
rm -vf install.sh &&  busybox wget -q -nd https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]

