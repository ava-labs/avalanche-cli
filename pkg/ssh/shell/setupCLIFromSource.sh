#!/usr/bin/env bash

#name: install avalanche-cli from source
cd ~
rm -rf avalanche-cli
git clone --single-branch -b {{ cliBranch }} https://github.com/ava-labs/avalanche-cli 
cd avalanche-cli
bash -i -c ./scripts/build.sh
cp bin/avalanche ~/bin/avalanche
