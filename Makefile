all: examples

examples: alohacam

alohacam:
	$(MAKE) -C examples/alohacam
	mv examples/alohacam/alohacam .

generate:
	go generate -x ./...

get:
	go get -d -v ./...


.PHONY: alohacam examples generate get
