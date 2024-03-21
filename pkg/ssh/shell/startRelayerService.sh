#!/usr/bin/env bash
set -e
sudo systemctl enable awm-relayer
sudo systemctl start awm-relayer
