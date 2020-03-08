This package provides an ARM assembly implementation of AES-128 CTR mode. It is
drop-in compatible with the Go's `crypto/aes` package (which currently includes
optimized implementations for several architectures, but not `arm`). The
assembly is derived from
[Nettle](https://github.com/gnutls/nettle/tree/master/arm), but translated to
Go's assembly dialect and with some modifications to account for reserved
registers.
