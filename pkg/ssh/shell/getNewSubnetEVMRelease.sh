#!/usr/bin/env bash
set -e
#name:TASK [download new subnet EVM release] 
busybox wget "{{ .SubnetEVMReleaseURL }}" -O "{{ .SubnetEVMArchive }}"
#name:TASK [unpack new subnet EVM release] 
tar xvf "{{ .SubnetEVMArchive}}"
