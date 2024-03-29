#!/usr/bin/env bash
# install gcc
echo "ensuring that gcc is installed ..."
while true; do
    gcc --version >/dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "GCC is installed!"
        break
    fi

    echo "GCC is not installed. Trying to install..."
    DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc

    if [ $? -neq 0 ]; then
        echo "Failed to install GCC. Retrying in 10 seconds..."
        sleep 10
    fi
done
# install go
echo "ensuring that go is installed ..."
go version || sudo snap install go --classic
