#!/usr/bin/env bash

if ! [[ "$0" =~ scripts/coverage.sh ]]; then
  echo "script must be run from the repository root"
  exit 1
fi

coverage_dir=$(PWD)/coverage
merged_coverage_dir=$coverage_dir/merged
merged_coverage_file=$merged_coverage_dir/merged.txt

echo "Recreating merged coverage directory..."
rm -rf ${merged_coverage_dir}
mkdir -p ${merged_coverage_dir}

echo "Generating coverage report in text format..."
included_packages=$(go list ./... | grep -v /tests/ | grep -v '/sdk/') # not including 'tests' and 'sdk'
go tool covdata merge -i=$coverage_dir/e2e,$coverage_dir/ut -o=$merged_coverage_dir -pkg=${included_packages//$'\n'/,}
go tool covdata textfmt -i=$merged_coverage_dir -o $merged_coverage_file
go tool cover -func $merged_coverage_file

echo "Checking total coverage..."

# TODO: coverage details will be output as a comment in the PR

go tool cover -func=$merged_coverage_file  | grep -e "total" | \
awk '{print "Total Coverage:", $3}'
