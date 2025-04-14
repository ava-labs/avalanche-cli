#!/usr/bin/env bash

# file name of code coverage report and it has to exist.
COVER_FILE=coverage.out
if [ ! -f $COVER_FILE ]; then
    echo "$COVER_FILE not found! Please run scripts/unit_test.sh first."
    exit 1
fi

# percentage threshold of code coverage required
COVERAGE_THRESHOLD=5.0

# print current test coverage from report
current_test_coverage=$(go tool cover -func=$COVER_FILE  | grep -e "total" | awk '{print $3}')
echo "Current test coverage is $current_test_coverage"

go tool cover -func=$COVER_FILE  | grep -e "total" | \
awk -v coverageThreshold=$COVERAGE_THRESHOLD '{if (($3 - 0.0) < coverageThreshold) \
  {print "Coverage is less than", coverageThreshold, "%. Checking Failed"; exit 1} \
  else \
  {print "Coverage is greater than or equal to", coverageThreshold, "%. Checking Passed."; exit 0}}'
