#!/usr/bin/env bash
#name:TASK [install gcc if not available]
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
#name:TASK [install go]
install_go() {
  ARCH=amd64
  [[ "$(uname -m)" == "aarch64" ]] && ARCH=arm64
  GOFILE="go{{ .GoVersion }}.linux-$ARCH.tar.gz"
  cd
  sudo rm -rf $GOFILE go
  wget -q -nv https://go.dev/dl/$GOFILE
  tar xfz $GOFILE
  echo >> ~/.bashrc
  echo export PATH=\$PATH:~/go/bin:~/bin >> ~/.bashrc
  echo export CGO_ENABLED=1 >> ~/.bashrc
}
go version || install_go
