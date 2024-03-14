#!/usr/bin/env bash
set -e
#name:TASK [install rust]
install_rust() {
  curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s - -y
  echo >> ~/.bashrc
  echo export PATH=\$PATH:~/.cargo/bin >> ~/.bashrc
}
cargo version || install_rust
