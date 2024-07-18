#!/usr/bin/env bash
set -e
# go to contracts dir
base_dir=$(dirname $0)/..
cd $base_dir/pkg/contract/contracts/
# prepare build env
touch foundry.toml
echo @openzeppelin/contracts@4.8.1/=lib/openzeppelin-contracts/contracts/ > remappings.txt
[ ! -d lib/openzeppelin-contracts ] && git clone https://github.com/OpenZeppelin/openzeppelin-contracts lib/openzeppelin-contracts -b v4.8.1
# build 
forge build --extra-output-files bin
mkdir -p bin
cp out/Token.sol/Token.bin bin
