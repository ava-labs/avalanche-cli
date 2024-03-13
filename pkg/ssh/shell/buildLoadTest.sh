#!/usr/bin/env bash
# get most updated load test repo
echo "getting load test repo ..."
git -C {{ .LoadTestRepoDir }} pull || git clone {{ .LoadTestRepo }}
{{if .CheckoutCommit }}
cd {{ .RepoDirName}}; git checkout {{ .LoadTestGitCommit}}
{{end}}
echo "successfully built load test binary!"
