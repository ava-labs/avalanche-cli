#!/usr/bin/env bash
set -e
#name:TASK [upsize disk]
sudo partprobe
sudo growpart /dev/nvme0n1 1 || sudo growpart /dev/sda 1
ROOT_DEVICE=$(df / | awk 'NR==2 {print $1}')
sudo resize2fs $ROOT_DEVICE
