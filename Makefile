CGO_ENABLED ?= 0
GOARM ?= 7
GOARCH ?= arm
GOOS ?= linux

# Allow setting additional go build flags, e.g. `make GOFLAGS=-tags=dev`
export GOFLAGS ?=

GIT_REVISION_ID := $(shell git describe --always --dirty)
BUILD_DATE := $(shell date)

all: examples

examples: alohacam

alohacam: generate
	CGO_ENABLED=$(CGO_ENABLED) GOARM=$(GOARM) GOARCH=$(GOARCH) GOOS=$(GOOS) \
		go build -ldflags '-s -w -X main.GitRevisionId=$(GIT_REVISION_ID) -X "main.BuildDate=$(BUILD_DATE)"' \
			-v -o alohacam \
			./examples/alohacam

generate:
	go generate -x ./...

get:
	go get -d -v ./...


.PHONY: alohacam examples generate get
