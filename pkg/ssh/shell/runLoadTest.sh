#!/usr/bin/env bash
# run load test command
install -g ubuntu  -o ubuntu -d `dirname {{ .LoadTestResultFile }}`
if [ -e {{ .LoadTestResultFile }} ]; then
  rm {{ .LoadTestResultFile }}
fi
nohup {{ .LoadTestCommand }} > {{ .LoadTestResultFile }} 2>&1 &
exit
