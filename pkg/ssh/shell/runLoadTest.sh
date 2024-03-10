#!/usr/bin/env bash
echo "running load test ..."
# export all variables in clusterInfo.yaml
yq --version || sudo snap install yq
while read -r key val; do
	eval "export $key=${val}"
done < <(yq '.[] | key + " " + .' clusterInfo.yaml)

# run load test command
echo {{ .LoadTestCommand }}
eval {{ .LoadTestCommand }}