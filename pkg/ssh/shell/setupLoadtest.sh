#!/usr/bin/env bash
# get most updated load test repo
echo "getting load test repo ..."
git -C {{ .LoadTestRepoDir }} pull || git clone {{ .LoadTestRepo }}
# install gcc
echo "ensuring that gcc is installed ..."
gcc --version || DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc
# install go
echo "ensuring that go is installed ..."
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
echo "building load test binary ..."
# run load test build command
eval {{ .LoadTestPath }}
echo "running load test ..."
# run load test command
eval {{ .LoadTestCommand }}