#!/usr/bin/env bash

go test -v -coverprofile=coverage.out $(go list ./... | grep -v /tests/ | grep -v '/sdk/')
#go tool cover -func=coverage.out
