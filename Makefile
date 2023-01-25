# SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

REGISTRY              := eu.gcr.io/gardener-project
EXECUTABLE            := net-gauger
PROJECT               := github.com/gardener/network-traffic-gauger
IMAGE_REPOSITORY      := $(REGISTRY)/gardener/network-traffic-gauger
REPO_ROOT             := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))
VERSION               := $(shell cat VERSION)
IMAGE_TAG             := $(VERSION)
EFFECTIVE_VERSION     := $(VERSION)-$(shell git rev-parse HEAD)
GOARCH                := amd64

.PHONY: revendor
revendor:
	@GO111MODULE=on go mod vendor
	@GO111MODULE=on go mod tidy


.PHONY: check
check: $(GOIMPORTS)
	go vet ./...

.PHONY: format
format:
	@$(REPO_ROOT)/hack/format.sh ./cmd ./pkg

.PHONY: build
build:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) GO111MODULE=on go build -o $(EXECUTABLE) \
        -mod=vendor \
	    -ldflags "-X 'main.Version=$(EFFECTIVE_VERSION)' -X 'main.ImageTag=$(IMAGE_TAG)'"\
	    ./cmd/net-gauger

.PHONY: build-local
build-local:
	@CGO_ENABLED=1 GO111MODULE=on go build -o $(EXECUTABLE) \
	    -race \
        -mod=vendor \
	    -ldflags "-X 'main.Version=$(EFFECTIVE_VERSION)' -X 'main.ImageTag=$(IMAGE_TAG)'"\
	    ./cmd/net-gauger


.PHONY: release
release:
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(GOARCH) GO111MODULE=on go build -o $(EXECUTABLE) \
        -mod=vendor \
        -ldflags "-w -X 'main.Version=$(EFFECTIVE_VERSION)' -X 'main.ImageTag=$(IMAGE_TAG)'"\
	    ./cmd/net-gauger

.PHONY: test
test:
	GO111MODULE=on go test -mod=vendor ./pkg/...

.PHONY: verify
verify: check format test

.PHONY: install-requirements
install-requirements:
	@go install -mod=vendor $(REPO_ROOT)/vendor/golang.org/x/tools/cmd/goimports

.PHONY: prepare-default-image
prepare-default-image:
	@echo "$(IMAGE_REPOSITORY):$(EFFECTIVE_VERSION)" >$(REPO_ROOT)/pkg/deploy/DEFAULT_IMAGE

.PHONY: docker-images
docker-images:
	@docker build -t $(IMAGE_REPOSITORY):$(IMAGE_TAG) -f Dockerfile .
