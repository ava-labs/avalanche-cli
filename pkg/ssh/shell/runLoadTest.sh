#!/usr/bin/env bash
# run load test command
mkdir -p `dirname {{ .LoadTestResultFile }}`
if [ -e {{ .LoadTestResultFile }} ]; then
  rm {{ .LoadTestResultFile }}
fi
nohup {{ .LoadTestCommand }} > {{ .LoadTestResultFile }} 2>&1 &
exit
