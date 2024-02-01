#!/usr/bin/env bash
#name:TASK [start node - start avalanchego] 
nohup /home/ubuntu/avalanche-node/avalanchego --config-file=/home/ubuntu/.avalanchego/configs/node.json </dev/null &>/dev/null &
sleep 2
