CGO_ENABLED ?= 0
GOARM ?= 7
GOARCH ?= "arm"
GOOS ?= "linux"

all: examples

examples: demo iot

demo:
	cd examples/demo && go generate
	CGO_ENABLED=$(CGO_ENABLED) GOARM=$(GOARM) GOARCH=$(GOARCH) GOOS=$(GOOS) \
		go build -ldflags "-s -w" -o examples/demo/demo -v \
			examples/demo/handlers.go \
			examples/demo/main.go \
			examples/demo/statics.go \
			examples/demo/templates.go

iot:
	cd examples/demo && go generate
	CGO_ENABLED=$(CGO_ENABLED) GOARM=$(GOARM) GOARCH=$(GOARCH) GOOS=$(GOOS) \
		go build -ldflags "-s -w" -o examples/iot/iot -v \
			github.com/lanikailabs/alohartc/examples/iot

.PHONY: demo examples iot
