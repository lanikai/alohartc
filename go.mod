module github.com/lanikai/alohartc

go 1.12

require (
	github.com/gorilla/websocket v1.4.0
	github.com/lanikai/alohartc/internal/dtls v0.0.0-00010101000000-000000000000
	github.com/lanikai/oahu/api v0.0.0-20190703205954-e5008c1038bd
	github.com/nareix/joy4 v0.0.0-20181022032202-3ddbc8f9d431
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.2.2
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb
)

replace github.com/lanikai/alohartc/internal/dtls => ./internal/dtls
