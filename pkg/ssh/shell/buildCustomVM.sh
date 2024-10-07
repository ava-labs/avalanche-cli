#!/usr/bin/env bash

if [ -d {{ .CustomVMRepoDir }} ]; then
  rm -rf {{ .CustomVMRepoDir }}
fi
mkdir -p {{ .CustomVMRepoDir }}

# prepare build script
cd {{ .CustomVMRepoDir }}
cat <<EOF >>build.sh
#!/usr/bin/env bash
set -x
git init -q
git remote add origin {{ .CustomVMRepoURL }}
git fetch --depth 1 origin {{ .CustomVMBranch }} -q
git checkout {{ .CustomVMBranch }}
chmod +x {{ .CustomVMBuildScript }}
./{{ .CustomVMBuildScript }} {{ .VMBinaryPath }}
echo {{ .VMBinaryPath }} [ok]
EOF
chmod +x build.sh

#prepare build Dockerfile
cat <<EOF >>Dockerfile
FROM golang:{{ .GoVersion }}-bullseye AS builder
RUN apt-get update
RUN apt-get install -y git ca-certificates 
RUN mkdir -p {{ .CustomVMRepoDir }}
WORKDIR {{ .CustomVMRepoDir }}
COPY build.sh .
RUN ./build.sh

FROM scratch AS vm
COPY --from=builder {{ .VMBinaryPath }} /
EOF

VMBinaryDir=$(dirname {{ .VMBinaryPath }})
# build
docker buildx build --target=vm --output type=local,dest=${VMBinaryDir} .





