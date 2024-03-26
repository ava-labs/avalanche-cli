#!/usr/bin/env bash
# run load test command
if [ -e {{ .LoadTestResultFile }} ]; then
  rm {{ .LoadTestResultFile }}
fi
nohup {{ .LoadTestCommand }} > {{ .LoadTestResultFile }} 2>&1 &
exit
