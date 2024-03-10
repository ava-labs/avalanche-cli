#!/usr/bin/env bash
echo "running load test ..."
# export all variables in clusterInfo.yaml
while read -r key val; do
	eval "export $key=${val}"
done < <(yq '.[] | key + " " + .' clusterInfo.yaml)
echo {{ .LoadTestCommand }}
# run load test command
eval {{ .LoadTestCommand }}