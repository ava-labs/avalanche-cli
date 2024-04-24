#!/usr/bin/env bash
export PATH=$PATH:~/go/bin
mkdir -p ~/.avalanche-cli
busybox wget -q -nd https://raw.githubusercontent.com/ava-labs/avalanche-cli/main/scripts/install.sh
#name:TASK [modify permissions]
chmod 755 install.sh
#name:TASK [run install script]
./install.sh -n {{ .CLIVersion }}
