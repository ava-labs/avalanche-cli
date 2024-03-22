#!/usr/bin/env bash
set -e
#name:TASK [upsize disk] 
ROOT_DEVICE=$(df / | awk 'NR==2 {print $1}')
resize2fs $ROOT_DEVICE
