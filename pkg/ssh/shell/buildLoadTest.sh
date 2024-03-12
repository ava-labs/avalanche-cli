#!/usr/bin/env bash
# get most updated load test repo
echo "getting load test repo ..."
git -C {{ .LoadTestRepoDir }} pull || git clone {{ .LoadTestRepo }}
echo "building load test binary ..."
# run load test build command
eval {{ .LoadTestPath }}
echo "successfully built load test binary!"
