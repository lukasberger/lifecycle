export GO111MODULE = on
GOCMD?=go
GOENV=GOOS=linux GOARCH=amd64 CGO_ENABLED=0
GOBUILD=$(GOCMD) build -mod=vendor -ldflags "-X 'github.com/buildpack/lifecycle/cmd.buildVersion=$(LIFECYCLE_BUILD_VERSION)'"
GOTEST=$(GOCMD) test -mod=vendor
LIFECYCLE_VERSION?=0.0.0
LIFECYCLE_BUILD_VERSION?=$(LIFECYCLE_VERSION)+$$(git rev-parse --short HEAD)
ARCHIVE_NAME=lifecycle-v$(LIFECYCLE_VERSION)+linux.x86-64

all: test build package
build:
	mkdir -p ./out/$(ARCHIVE_NAME)
	$(GOENV) $(GOBUILD) -o ./out/$(ARCHIVE_NAME)/detector -a ./cmd/detector
	$(GOENV) $(GOBUILD) -o ./out/$(ARCHIVE_NAME)/restorer -a ./cmd/restorer
	$(GOENV) $(GOBUILD) -o ./out/$(ARCHIVE_NAME)/analyzer -a ./cmd/analyzer
	$(GOENV) $(GOBUILD) -o ./out/$(ARCHIVE_NAME)/builder -a ./cmd/builder
	$(GOENV) $(GOBUILD) -o ./out/$(ARCHIVE_NAME)/exporter -a ./cmd/exporter
	$(GOENV) $(GOBUILD) -o ./out/$(ARCHIVE_NAME)/cacher -a ./cmd/cacher
	$(GOENV) $(GOBUILD) -o ./out/$(ARCHIVE_NAME)/launcher -a ./cmd/launcher

imports:
	$(GOCMD) install -mod=vendor golang.org/x/tools/cmd/goimports
	test -z $$(goimports -l -w -local github.com/buildpack/lifecycle $$(find . -type f -name '*.go' -not -path "./vendor/*"))

format:
	test -z $$($(GOCMD) fmt ./...)

vet:
	$(GOCMD) vet $$($(GOCMD) list ./... | grep -v /testdata/)

test: unit acceptance

unit: format imports vet
	$(GOTEST) -v -count=1 ./...

acceptance: format imports vet
	$(GOTEST) -v -count=1 -tags=acceptance ./acceptance/...

clean:
	rm -rf ./out

package:
	chmod 755 ./out/${ARCHIVE_NAME}/*
	tar czf ./out/$(ARCHIVE_NAME).tgz -C out $(ARCHIVE_NAME) --owner=root --group=root
