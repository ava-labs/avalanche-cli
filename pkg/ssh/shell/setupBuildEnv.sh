#!/usr/bin/env bash
set -e
#name:TASK [install gcc if not available]
export DEBIAN_FRONTEND=noninteractive
until sudo apt-get -y update -o DPkg::Lock::Timeout=120; do sleep 1 && echo "Try again"; done
until sudo apt-get -y install -o DPkg::Lock::Timeout=120 gcc; do sleep 1 && echo "Try again"; done
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
