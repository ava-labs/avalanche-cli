#!/usr/bin/env bash

set -ex

coverage_dir=$PWD/coverage/ut # should be under the same parent folder as e2e coverage dir

echo "Re-creating unit test coverage directory: $coverage_dir"
rm -rf $coverage_dir
mkdir -p $coverage_dir

go test -cover -v  $(go list ./... | grep -v /tests/ | grep -v '/sdk/') -args -test.gocoverdir=$coverage_dir

