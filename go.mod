module github.com/lanikai/alohartc

go 1.13

require (
	github.com/gorilla/websocket v1.4.0
	github.com/lanikai/alohartc/internal/dtls v0.0.0-20190830191728-a3f721372687
	github.com/lanikai/oahu/api v0.0.0-20190703205954-e5008c1038bd
	github.com/nareix/joy4 v0.0.0-20181022032202-3ddbc8f9d431
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.2.2
	golang.org/x/crypto v0.0.0-20190923035154-9ee001bba392 // indirect
	golang.org/x/sys v0.0.0-20190922100055-0a153f010e69
	golang.org/x/xerrors v0.0.0-20190717185122-a985d3407aa7
)

replace github.com/lanikai/alohartc/internal/dtls => ./internal/dtls
