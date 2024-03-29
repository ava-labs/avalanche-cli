#!/usr/bin/env bash
# delete existing repo directory if it exists
# choosing to delete and re-clone to avoid merge conflicts
if [ -d {{ .LoadTestRepoDir }} ]; then
  rm -r {{ .LoadTestRepoDir }}
fi
git clone {{ .LoadTestRepo }}
echo "getting load test repo ..."
cd {{ .LoadTestRepoDir }}
git fetch --depth 1 origin {{ .LoadTestBranch }} -q
git checkout {{ .LoadTestBranch }}
eval {{ .LoadTestPath }}
echo "successfully built load test binary!"
