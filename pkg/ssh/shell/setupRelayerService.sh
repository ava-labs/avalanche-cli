#!/usr/bin/env bash
set -e
~/bin/avalanche teleporter relayer prepareService
sudo cp ~/.avalanche-cli/services/awm-relayer/awm-relayer.service /etc/systemd/system/awm-relayer.service
sudo systemctl daemon-reload
