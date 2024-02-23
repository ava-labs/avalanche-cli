#!/usr/bin/env bash
git clone {{ .LoadTestRepo }}
sudo snap install go --classic
sudo apt install gcc
{{ .LoadTestPath }}
{{ .LoadTestCommand }}