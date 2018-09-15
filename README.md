WebRTC
======

Go package implementing a WebRTC native client

```
.
├── NOTES.md
├── README.md
├── client.key                 Client certificate private key (for now)
├── client.pem                 Client certificate (for now)
├── examples
│   └── client.go              Run this example for demo
├── internal                   Special Go directory for internal modules
│   └── srtp
│       ├── kdf.go
│       ├── kdf_test.go
│       ├── messages.go
│       └── srtp.go
├── peer_connection.go         Top-level WebRTC PeerConnection
├── stun.go                    Bare-bones STUN implementation for demo
├── stun_test.go
├── testdata                   Special Go directory for testdata
│   └── admiral.264
├── web                        Web content for demo
│   ├── static
│   │   └── js
│   │       └── adapter-latest.js
│   └── templates
│       └── index.html
└── x                          Experiments
    └── twobrowser             Browser-to-browser manual WebRTC call
        ├── callee.html
        ├── caller.html
        └── js
            └── adapter-latest.js
```
