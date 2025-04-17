#!/usr/bin/env bash

coverage_dir=$(PWD)/coverage
combined_coverage_file=$coverage_dir/combined.txt
COVERAGE_THRESHOLD=15.0 # percentage threshold of code coverage required

echo "Generating coverage report in text format..."
go tool covdata merge -i=$coverage_dir/e2e,$coverage_dir/ut -o=$coverage_dir
go tool covdata textfmt -i=./coverage -o $combined_coverage_file
go tool cover -func $combined_coverage_file

echo "Checking total coverage..."

# print current test coverage from report
current_test_coverage=$(go tool cover -func=$combined_coverage_file  | grep -e "total" | awk '{print $3}')
echo "Current test coverage is $current_test_coverage"

go tool cover -func=$combined_coverage_file  | grep -e "total" | \
awk -v coverageThreshold=$COVERAGE_THRESHOLD '{if (($3 - 0.0) < coverageThreshold) \
  {print "Coverage is less than", coverageThreshold, "%. Checking Failed"; exit 1} \
  else \
  {print "Coverage is greater than or equal to", coverageThreshold, "%. Checking Passed."; exit 0}}'
