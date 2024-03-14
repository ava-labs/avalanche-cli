#!/usr/bin/env bash
# get most updated load test repo
echo "getting load test repo ..."
git -C {{ .LoadTestRepoDir }} pull || git clone {{ .LoadTestRepo }}
{{if .CheckoutCommit }}
cd {{ .RepoDirName}}; git checkout {{ .LoadTestGitCommit}}
{{end}}
# install gcc
echo "ensuring that gcc is installed ..."
gcc --version || DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc
# install go
echo "ensuring that go is installed ..."
go version || sudo snap install go --classic
echo "building load test binary ..."
# run load test build command
eval {{ .LoadTestPath }}
echo "successfully built load test binary!"