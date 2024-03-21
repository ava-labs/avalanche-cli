#!/usr/bin/env bash
# install gcc
echo "ensuring that gcc is installed ..."
gcc --version || DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc
# install go
echo "ensuring that go is installed ..."
go version || sudo snap install go --classic
mkdir -p /home/ubuntu/loadtest-logs/
