#!/ust/bin/env bash
#name:TASK [install gcc if not available]
gcc --version || DEBIAN_FRONTEND=noninteractive sudo apt-get -y install gcc
#name:TASK [install go]
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
#name:TASK [install rust]
install_rust() {
  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s - -y
  echo >> ~/.bashrc
  echo export PATH=\$PATH:~/.cargo/bin >> ~/.bashrc 
}
cargo version || install_rust
