all: examples

examples: demo iot

demo:
	cd examples/demo && go generate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-s -w" -o examples/demo/demo -v \
				examples/demo/handlers.go \
				examples/demo/main.go \
				examples/demo/statics.go \
				examples/demo/templates.go

iot:
	cd examples/demo && go generate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 \
		    go build -ldflags "-s -w" -o examples/iot/iot -v \
		    github.com/lanikailabs/webrtc/examples/iot

.PHONY: demo examples iot
