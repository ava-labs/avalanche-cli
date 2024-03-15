#!/usr/bin/env bash
set -e
~/bin/avalanche teleporter relayer addSubnetToService {{ .SubnetName }} --cluster {{ .ClusterName }}
