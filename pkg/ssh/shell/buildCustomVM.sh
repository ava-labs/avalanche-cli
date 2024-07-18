#!/usr/bin/env bash

if [ -d {{ .CustomVMRepoDir }} ]; then
  rm -rf {{ .CustomVMRepoDir }}
fi

cd {{ .CustomVMRepoDir }}
git init -q
git remote add origin {{ .CustomVMRepoURL }}
git fetch --depth 1 origin {{ .CustomVMBranch }} -q
git checkout {{ .CustomVMBranch }}
chmod +x {{ .CustomVMBuildScript }}
./{{ .CustomVMBuildScript }} {{ .VMBinaryPath }}
echo {{ .VMBinaryPath }} [ok]


