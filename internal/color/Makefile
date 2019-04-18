SOURCES := converters_test.go converters.go yuyv_to_yuv420p.c yuyv_to_yuv420p.h

all: color-arm64.test color-arm5.test color-arm6.test color-arm7.test

color-arm64.test: $(SOURCES)
	CC=aarch64-linux-gnu-gcc GOARCH=arm64 GOOS=linux CGO_ENABLED=1 go test -c -o $@

color-arm5.test: $(SOURCES)
	CC=arm-linux-gnueabihf-gcc GOARCH=arm GOARM=5 GOOS=linux CGO_ENABLED=1 go test -c -o $@

color-arm6.test: $(SOURCES)
	CC=arm-linux-gnueabihf-gcc GOARCH=arm GOARM=6 GOOS=linux CGO_ENABLED=1 go test -c -o $@

color-arm7.test: $(SOURCES)
	CC=arm-linux-gnueabihf-gcc GOARCH=arm GOARM=7 GOOS=linux CGO_ENABLED=1 go test -c -o $@

clean:
	rm -f color-*.test

.PHONY: clean
