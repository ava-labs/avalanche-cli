#!/usr/bin/env bash
set -e
#name:TASK [upgrade avalanchego version] 
./avalanchego-installer.sh --version {{ .AvalancheGoVersion }}
