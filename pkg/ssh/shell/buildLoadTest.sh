#!/usr/bin/env bash
# delete existing repo directory if it exists
# choosing to delete and re-clone to avoid merge conflicts
if [ -d {{ .LoadTestRepoDir }} ]; then
  rm -r {{ .LoadTestRepoDir }}
fi
while true; do
    gcc --version >/dev/null 2>&1
    if [ $? -eq 0 ]; then
        echo "GCC is installed!"
        break
    fi

    echo "GCC is not installed. Trying to install..."
    DEBIAN_FRONTEND=noninteractive sudo apt-get -y -o DPkg::Lock::Timeout=120 install gcc

    if [ $? -neq 0 ]; then
        echo "Failed to install GCC. Retrying in 10 seconds..."
        sleep 10
    fi
done
git clone {{ .LoadTestRepo }}
echo "getting load test repo ..."
cd {{ .LoadTestRepoDir }}
git fetch --depth 1 origin {{ .LoadTestBranch }} -q
git checkout {{ .LoadTestBranch }}
eval {{ .LoadTestPath }}
echo "successfully built load test binary!"
