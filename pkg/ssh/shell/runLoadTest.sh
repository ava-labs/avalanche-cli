#!/usr/bin/env bash
# run load test command
if [ -e {{ .LoadTestResultFile }} ]; then
  rm {{ .LoadTestResultFile }}
fi
eval {{ .LoadTestCommand }} > {{ .LoadTestResultFile }} 2> /dev/null &
exit
