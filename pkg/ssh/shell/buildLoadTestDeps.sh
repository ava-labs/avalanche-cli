#!/usr/bin/env bash
# install gcc
echo "ensuring that gcc is installed ..."
export DEBIAN_FRONTEND=noninteractive
while ! gcc --version >/dev/null 2>&1; do
    echo "GCC is not installed. Trying to install..."
    sudo apt-get -y -o DPkg::Lock::Timeout=120 update
    sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc
    if [ $? -ne 0 ]; then
        echo "Failed to install GCC. Retrying in 10 seconds..."
        sleep 10
    fi
done
# install go
echo "ensuring that go is installed ..."
go version || sudo snap install go --classic
