#!/usr/bin/env bash

install_go() {
  GOFILE=go{{ .goVersion }}.linux-amd64.tar.gz
  cd ~
  sudo rm -rf $GOFILE go
  wget -nv https://go.dev/dl/$GOFILE
  tar xfz $GOFILE
  echo >> ~/.bashrc
  echo export PATH=\$PATH:~/go/bin:~/bin >> ~/.bashrc
  echo export CGO_ENABLED=1 >> ~/.bashrc
}

install_rust() {
  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s - -y
  echo >> ~/.bashrc
  echo export PATH=\$PATH:~/.cargo/bin >> ~/.bashrc 
}

#name: update apt data and install dependencies
DEBIAN_FRONTEND=noninteractive apt-get -y update
DEBIAN_FRONTEND=noninteractive apt-get -y install wget curl git sudo
#name: install gcc if not available
gcc --version || DEBIAN_FRONTEND=noninteractive apt-get -y install gcc
#name: install go
go version || install_go
#name: install rust
cargo version || install_rust
