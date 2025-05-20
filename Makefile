
#VERSIONS
LOCALSTACK_VERSION=3.0.2
#END OF VERSIONS

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

SHELL = /usr/bin/env bash -o pipefail
#.SHELLFLAGS = -ec
ROOT_DIR:=$(shell dirname $(realpath $(firstword $(MAKEFILE_LIST))))

#Detect OS params
UNAME := $(shell uname -s | tr A-Z a-z)
ARCH :=$(shell uname -m | tr A-Z a-z)
DOCKER:= $(shell command -v docker 2> /dev/null)
COLIMA:= $(shell command -v colima 2> /dev/null)

all: build

##@ Development

fmt: ## Run gofumpt against code.
	gofumpt -l -w .

vet: ## Run go vet against code.
	go vet ./...

test: fmt vet ## Run tests.
	go test -v ./... -coverprofile cover.out

lint: ## Run golangci-lint against code.
	golangci-lint run  --path-prefix=.
	gofumpt -w .
	
build: fmt vet ## Build avalanche CLI binary.
	scripts/build.sh

fast-build:
	scripts/build.sh

run: build ## Run avalanche CLI
	./bin/avalanche help

colima: ## check colima
ifndef COLIMA
	brew install colima
	brew install docker docker-compose
	brew install chipmk/tap/docker-mac-net-connect
	brew services start chipmk/tap/docker-mac-net-connect
endif
	
docker: colima ## check docker
ifndef DOCKER
	$(error "No docker in $(PATH), pls follow https://docs.docker.com/get-docker/ to install")	
endif

docker-build: docker ## Build docker image
	docker build . -t avalanche-cli

docker-e2e-build: docker build ## Build docker image for e2e
	docker build -f Dockerfile.release ./bin -t avalanche-cli
