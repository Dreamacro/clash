## Clash testing suit

### Protocol testing suit

* TCP pingpong test
* UDP pingpong test
* TCP large data test
* UDP large data test

### Protocols

- [x] Shadowsocks
  - [x] Normal
  - [x] ObfsHTTP
  - [x] ObfsTLS
  - [x] ObfsV2rayPlugin
- [x] Vmess
  - [x] Normal
  - [x] AEAD
  - [x] HTTP
  - [x] HTTP2
  - [x] TLS
  - [x] Websocket
  - [x] Websocket TLS
  - [x] gRPC
- [x] Trojan
  - [x] Normal
  - [x] gRPC
- [x] Snell
  - [x] Normal
  - [x] ObfsHTTP
  - [x] ObfsTLS

### Features

- [ ] DNS
  - [x] DNS Server
  - [x] FakeIP
  - [x] Host

### Command

Prerequisite

* docker (support Linux and macOS)

```
$ go test -p 1 -v
```
