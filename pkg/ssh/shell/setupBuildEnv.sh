#!/usr/bin/env bash
set -e
#name:TASK [install gcc if not available]
sudo apt-get -y -o DPkg::Lock::Timeout=120 update
gcc --version || DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc
#name:TASK [install go]
install_go() {
  local GOFILE
  if [[ "$(uname -m)" == "aarch64" ]]; then
    GOFILE="go{{ .GoVersion }}.linux-arm64.tar.gz"
  else
    GOFILE="go{{ .GoVersion }}.linux-amd64.tar.gz"
  fi
  cd ~
  sudo rm -rf $GOFILE go
  wget -q -nv https://go.dev/dl/$GOFILE
  tar xfz $GOFILE
  echo >> ~/.bashrc
  echo export PATH=\$PATH:~/go/bin:~/bin >> ~/.bashrc
  echo export CGO_ENABLED=1 >> ~/.bashrc
}
go version || install_go
