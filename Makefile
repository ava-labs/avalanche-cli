# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

SHELL = /usr/bin/env bash -o pipefail
#.SHELLFLAGS = -ec
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

all: build

##@ Development

fmt: ## Run gofumpt against code.
	gofumpt -l -w .

vet: ## Run go vet against code.
	go vet ./...

test: fmt vet envtest ## Run tests.
	go test -v ./... -coverprofile cover.out

lint: ## Run golangci-lint against code.
	golangci-lint run  --path-prefix=.
	
build: fmt vet ## Build avalanche CLI binary.
	scripts/build.sh

run: build ## Run avalanche CLI
	./bin/avalanche help
	
docker: ## check docker
	docker --version || (echo "docker is not installed. Pls follow https://docs.docker.com/get-docker/" && exit 1)

docker-build: docker ## Build docker image
	docker build . -t avalanche-cli
