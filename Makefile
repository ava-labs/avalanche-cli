
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

#Detect OS
UNAME := $(shell uname -s | tr A-Z a-z)
ARCH :=$(shell uname -m | tr A-Z a-z)
LOCALSTACK := $(shell command -v localstack 2> /dev/null)
DOCKER:= $(shell command -v docker 2> /dev/null)
COLIMA:= $(shell command -v colima 2> /dev/null)

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

colima: ## check colima
ifndef COLIMA
brew install colima
brew install docker
endif
	
docker: colima ## check docker
ifndef DOCKER
$(error "No docker in $(PATH), pls follow https://docs.docker.com/get-docker/ to install")	
endif

docker-build: docker ## Build docker image
	docker build . -t avalanche-cli

docker-pull-focal: docker ## Pull docker image
	docker pull ubuntu:focal
	docker tag ubuntu:focal localstack-ec2/ubuntu-focal-ami:ami-000001

localstack: docker
ifndef LOCALSTACK
	curl -Lo localstack-cli-$(LOCALSTACK_VERSION)-$(UNAME)-$(ARCH)-onefile.tar.gz https://github.com/localstack/localstack-cli/releases/download/v$(LOCALSTACK_VERSION)/localstack-cli-$(LOCALSTACK_VERSION)-$(UNAME)-$(ARCH)-onefile.tar.gz
	sudo tar xvzf localstack-cli-$(LOCALSTACK_VERSION)-$(UNAME)-*-onefile.tar.gz -C /usr/local/bin
	rm -f localstack-cli-$(LOCALSTACK_VERSION)-$(UNAME)-*-onefile.tar.gz
endif

localstack-start: localstack docker-pull-focal
	localstack start -d
	localstack wait -t 30
	export AWS_ENDPOINT_URL=http://localhost:4566
	export AWS_REGION=us-east-1
	export AWS_DEFAULT_REGION=us-east-1
	export AWS_ACCESS_KEY_ID=test
	export AWS_SECRET_ACCESS_KEY=test
 
localstack-stop: localstack
	localstack stop
