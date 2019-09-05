all: alohartcd

alohartcd:
	$(MAKE) -C cmd/alohartcd

generate:
	go generate -x ./...

get:
	go get -d -v ./...


.PHONY: alohartcd generate get
