#!/usr/bin/env bash
echo "running load test ..."
# run load test command
echo {{ .LoadTestCommand }}
eval {{ .LoadTestCommand }}