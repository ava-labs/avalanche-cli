#!/usr/bin/env bash
export PATH=$PATH:~/go/bin
cd ~
rm -rf avalanche-cli
git clone --single-branch -b {{ .CliBranch }} https://github.com/ava-labs/avalanche-cli 
cd avalanche-cli
./scripts/build.sh
cp bin/avalanche ~/bin/avalanche
