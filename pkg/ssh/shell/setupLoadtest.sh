#!/usr/bin/env bash
# get most updated load test repo
git -C {{ .LoadTestRepoDir }} pull || git clone {{ .LoadTestRepo }}
# install gcc
gcc --version || DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc
#name:TASK [install go]
# install go
install_go() {
  GOFILE=go{{ .GoVersion }}.linux-amd64.tar.gz
  cd ~
  sudo rm -rf $GOFILE go
  wget -nv https://go.dev/dl/$GOFILE
  tar xfz $GOFILE
  echo >> ~/.bashrc
  echo export PATH=\$PATH:~/go/bin:~/bin >> ~/.bashrc
  echo export CGO_ENABLED=1 >> ~/.bashrc
}
go version || install_go
# run load test build command
eval {{ .LoadTestPath }}
# run load test command
eval {{ .LoadTestCommand }}