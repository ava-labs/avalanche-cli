#!/usr/bin/env bash
set -e
#name:TASK [install gcc if not available]
gcc --version || DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc
