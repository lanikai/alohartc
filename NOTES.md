* There are two STUN binding requests -- first, one from caller to callee,
  followed by one from callee to caller. Both occur on same port tuple.

* The message integrity check on the STUN binding request uses the ice-pwd
  value verbatim as the HMAC-SHA1 key. The length (byte 3, zero indexed) must
  be changed prior to computing the HMAC. The length is the number of bytes
  in the STUN message _after_ the header, which is 20 (0x14) bytes. For the
  HMAC computating, the length must be set to include the message integrity
  check, but the actual bytes over which the HMAC is computed do not include
  the message integrity check bytes.

* DTLS Client Hello must be sent from and to same port tuple as STUN binding
  request and response.

* OpenSSL 1.1.0h development DTLS 1.2 server:

    DYLD_LIBRARY_PATH=/usr/local/stow/openssl-1.1.0h/lib /usr/local/stow/openssl-1.1.0h/bin/openssl s_server -dtls1_2 -msg -mtu 1500 -named_curve prime256v1 -key certs/ecdsakey.key -cert certs/ecdsacert.pem -use_srtp SRTP_AES128_CM_SHA1_80

  Then run pure DTLS 1.2 client:

    go run examples/simple_client.go


## Domain ideas

RTC Logic (rtclogic.com)

RTC Ware (rtcware.com)

uRTC (micrortc.com)

Artcy
Artci
seertc
