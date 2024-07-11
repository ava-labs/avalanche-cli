#!/usr/bin/env bash

if [ -d {{ .CustomVMRepoDir }} ]; then
  rm -rf {{ .LoadTestRepoDir }}
fi

cd {{ .CustomVMRepoDir }}
git remote init -q
git remote add origin {{ .CustomVMRepoURL }}
git fetch --depth 1 origin {{ .CustomVMBranch }} -q
git checkout {{ .CustomVMBranch }}
chmod +x {{ .CustomVMBuildScript }}
./{{ .CustomVMBuildScript }} {{ .VMBinaryPath }}
echo {{ .VMBinaryPath }} [ok]


