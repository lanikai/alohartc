all: examples

examples: demo

demo:
	cd examples/demo && go generate
	CGO_ENABLED=0 GOOS=linux GOARCH=arm GOARM=7 go build -ldflags "-s -w" -o examples/demo/demo -v \
				examples/demo/handlers.go \
				examples/demo/main.go \
				examples/demo/statics.go \
				examples/demo/templates.go

.PHONY: demo examples
