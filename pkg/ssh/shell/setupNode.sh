#!/usr/bin/env bash
export PATH=$PATH:~/go/bin
mkdir -p ~/.avalanche-cli
{{ if .IsE2E }}
echo "E2E detected"
echo "CLI Version: {{ .CLIVersion }}"
export DEBIAN_FRONTEND=noninteractive
sudo apt-get -y update && sudo apt-get -y install busybox-static software-properties-common 
sudo add-apt-repository -y ppa:longsleep/golang-backports
sudo apt-get -y update && sudo apt-get -y dist-upgrade && sudo apt-get -y install ca-certificates curl gcc git golang-go
sudo install -m 0755 -d /etc/apt/keyrings && sudo curl -fsSL https://download.docker.com/linux/ubuntu/gpg -o /etc/apt/keyrings/docker.asc && sudo chmod a+r /etc/apt/keyrings/docker.asc
echo deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/ubuntu $(. /etc/os-release && echo \"$VERSION_CODENAME\") stable | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
sudo apt-get -y update && sudo apt-get -y install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-compose
sudo usermod -aG docker ubuntu
sudo chgrp ubuntu /var/run/docker.sock
sudo chmod +rw /var/run/docker.sock
{{ end }}
cd /tmp
rm -vf install.sh &&  busybox wget -q -nd https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]

