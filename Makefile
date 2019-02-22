CGO_ENABLED ?= 0
GOARM ?= 7
GOARCH ?= "arm"
GOOS ?= "linux"

GIT_REVISION_ID := $(shell git describe --always --dirty)
BUILD_DATE := $(shell date)

all: examples

examples: alohacam

alohacam:
	cd examples/alohacam && go generate
	CGO_ENABLED=$(CGO_ENABLED) GOARM=$(GOARM) GOARCH=$(GOARCH) GOOS=$(GOOS) \
		go build -ldflags '-s -w -X main.GitRevisionId=$(GIT_REVISION_ID) -X "main.BuildDate=$(BUILD_DATE)"' \
			-v -o alohacam \
			examples/alohacam/handlers.go \
			examples/alohacam/main.go \
			examples/alohacam/statics.go \
			examples/alohacam/templates.go


.PHONY: alohacam examples
