<h1 align="center">
  <img src="https://github.com/Dreamacro/clash/raw/master/docs/logo.png" alt="Clash" width="200">
  <br>Clash<br>
</h1>

<h4 align="center">A rule-based tunnel in Go.</h4>

<p align="center">
  <a href="https://travis-ci.org/Dreamacro/clash">
    <img src="https://img.shields.io/travis/Dreamacro/clash.svg?style=flat-square"
         alt="Travis-CI">
  </a>
  <a href="https://goreportcard.com/report/github.com/Dreamacro/clash">
    <img src="https://goreportcard.com/badge/github.com/Dreamacro/clash?style=flat-square">
  </a>
  <a href="https://github.com/Dreamacro/clash/releases">
    <img src="https://img.shields.io/github/release/Dreamacro/clash/all.svg?style=flat-square">
  </a>
</p>

## Features

- HTTP/HTTPS and SOCKS protocol
- Surge like configuration
- GeoIP rule support
- Support Vmess/Shadowsocks/Socks5
- Support for Netfilter TCP redirect

## Install

You can build from source:

```sh
go get -u -v github.com/Dreamacro/clash
```

Pre-built binaries are available: [release](https://github.com/Dreamacro/clash/releases)

Requires Go >= 1.13.

Checkout Clash version:

```sh
clash -v
```

## Daemon

Unfortunately, there is no native elegant way to implement golang's daemon.

So we can use third-party daemon tools like pm2, supervisor, and so on.

In the case of [pm2](https://github.com/Unitech/pm2), we can start the daemon this way:

```sh
pm2 start clash
```

If you have Docker installed, you can run clash directly using `docker-compose`.

[Run clash in docker](https://github.com/Dreamacro/clash/wiki/Run-clash-in-docker)

## Config

The default configuration directory is `$HOME/.config/clash`

The name of the configuration file is `config.yaml`

If you want to use another directory, you can use `-d` to control the configuration directory

For example, you can use the current directory as the configuration directory

```sh
clash -d .
```

Below is a simple demo configuration file:

```yml
# port of HTTP
port: 7890

# port of SOCKS5
socks-port: 7891

# redir port for Linux and macOS
# redir-port: 7892

allow-lan: false

# Only applicable when setting allow-lan to true
# "*": bind all IP addresses
# 192.168.122.11: bind a single IPv4 address
# "[aaaa::a8aa:ff:fe09:57d8]": bind a single IPv6 address
bind-address: "*"

# Rule / Global/ Direct (default is Rule)
mode: Rule

# set log level to stdout (default is info)
# info / warning / error / debug / silent
log-level: info

# A RESTful API for clash
external-controller: 127.0.0.1:9090

# you can put the static web resource (such as clash-dashboard) to a directory, and clash would serve in `${API}/ui`
# input is a relative path to the configuration directory or an absolute path
# external-ui: folder

# Secret for RESTful API (Optional)
# secret: ""

# experimental feature
experimental:
  ignore-resolve-fail: true # ignore dns resolve fail, default value is true

# authentication of local SOCKS5/HTTP(S) server
# authentication:
#  - "user1:pass1"
#  - "user2:pass2"

# # experimental hosts, support wildcard (e.g. *.clash.dev Even *.foo.*.example.com)
# # static domain has a higher priority than wildcard domain (foo.example.com > *.example.com)
# hosts:
#   '*.clash.dev': 127.0.0.1
#   'alpha.clash.dev': '::1'

# dns:
  # enable: true # set true to enable dns (default is false)
  # ipv6: false # default is false
  # listen: 0.0.0.0:53
  # enhanced-mode: redir-host # or fake-ip
  # # fake-ip-range: 198.18.0.1/16 # if you don't know what it is, don't change it
  # nameserver:
  #   - 114.114.114.114
  #   - tls://dns.rubyfish.cn:853 # dns over tls
  #   - https://1.1.1.1/dns-query # dns over https
  # fallback: # concurrent request with nameserver, fallback used when GEOIP country isn't CN
  #   - tcp://1.1.1.1
  # fallback-filter:
  #   geoip: true # default
  #   ipcidr: # ips in these subnets will be considered polluted
  #     - 240.0.0.0/4

Proxy:

# shadowsocks
# The supported ciphers(encrypt methods):
#   aes-128-gcm aes-192-gcm aes-256-gcm
#   aes-128-cfb aes-192-cfb aes-256-cfb
#   aes-128-ctr aes-192-ctr aes-256-ctr
#   rc4-md5 chacha20 chacha20-ietf xchacha20
#   chacha20-ietf-poly1305 xchacha20-ietf-poly1305
- { name: "ss1", type: ss, server: server, port: 443, cipher: chacha20-ietf-poly1305, password: "password", udp: true }

# old obfs configuration remove after prerelease
- name: "ss2"
  type: ss
  server: server
  port: 443
  cipher: chacha20-ietf-poly1305
  password: "password"
  plugin: obfs
  plugin-opts:
    mode: tls # or http
    # host: bing.com

- name: "ss3"
  type: ss
  server: server
  port: 443
  cipher: chacha20-ietf-poly1305
  password: "password"
  plugin: v2ray-plugin
  plugin-opts:
    mode: websocket # no QUIC now
    # tls: true # wss
    # skip-cert-verify: true
    # host: bing.com
    # path: "/"
    # headers:
    #   custom: value

# vmess
# cipher support auto/aes-128-gcm/chacha20-poly1305/none
- { name: "vmess", type: vmess, server: server, port: 443, uuid: uuid, alterId: 32, cipher: auto }
# with tls
- { name: "vmess", type: vmess, server: server, port: 443, uuid: uuid, alterId: 32, cipher: auto, tls: true }
# with tls and skip-cert-verify
- { name: "vmess", type: vmess, server: server, port: 443, uuid: uuid, alterId: 32, cipher: auto, tls: true, skip-cert-verify: true }
# with ws-path and ws-headers
- { name: "vmess", type: vmess, server: server, port: 443, uuid: uuid, alterId: 32, cipher: auto, network: ws, ws-path: /path, ws-headers: { Host: v2ray.com } }
# with ws + tls
- { name: "vmess", type: vmess, server: server, port: 443, uuid: uuid, alterId: 32, cipher: auto, network: ws, ws-path: /path, tls: true }

# socks5
- { name: "socks", type: socks5, server: server, port: 443 }
# socks5 with authentication
- { name: "socks", type: socks5, server: server, port: 443, username: "username", password: "password" }
# with tls
- { name: "socks", type: socks5, server: server, port: 443, tls: true }
# with tls and skip-cert-verify
- { name: "socks", type: socks5, server: server, port: 443, tls: true, skip-cert-verify: true }

# http
- { name: "http", type: http, server: server, port: 443 }
# http with authentication
- { name: "http", type: http, server: server, port: 443, username: "username", password: "password" }
# with tls (https)
- { name: "http", type: http, server: server, port: 443, tls: true }
# with tls (https) and skip-cert-verify
- { name: "http", type: http, server: server, port: 443, tls: true, skip-cert-verify: true }

Proxy Group:
# url-test select which proxy will be used by benchmarking speed to a URL.
- { name: "auto", type: url-test, proxies: ["ss1", "ss2", "vmess1"], url: "http://www.gstatic.com/generate_204", interval: 300 }

# fallback select an available policy by priority. The availability is tested by accessing an URL, just like an auto url-test group.
- { name: "fallback-auto", type: fallback, proxies: ["ss1", "ss2", "vmess1"], url: "http://www.gstatic.com/generate_204", interval: 300 }

# load-balance: The request of the same eTLD will be dial on the same proxy.
- { name: "load-balance", type: load-balance, proxies: ["ss1", "ss2", "vmess1"], url: "http://www.gstatic.com/generate_204", interval: 300 }

# select is used for selecting proxy or proxy group
# you can use RESTful API to switch proxy, is recommended for use in GUI.
- { name: "Proxy", type: select, proxies: ["ss1", "ss2", "vmess1", "auto"] }

Rule:
- DOMAIN-SUFFIX,google.com,auto
- DOMAIN-KEYWORD,google,auto
- DOMAIN,google.com,auto
- DOMAIN-SUFFIX,ad.com,REJECT
- IP-CIDR,127.0.0.0/8,DIRECT
# rename SOURCE-IP-CIDR and would remove after prerelease
- SRC-IP-CIDR,192.168.1.201/32,DIRECT
- GEOIP,CN,DIRECT
- DST-PORT,80,DIRECT
- SRC-PORT,7777,DIRECT
# FINAL would remove after prerelease
# you also can use `FINAL,Proxy` or `FINAL,,Proxy` now
- MATCH,auto
```

## Documentations
https://clash.gitbook.io/

## Thanks

[riobard/go-shadowsocks2](https://github.com/riobard/go-shadowsocks2)

[v2ray/v2ray-core](https://github.com/v2ray/v2ray-core)

## License

[![FOSSA Status](https://app.fossa.io/api/projects/git%2Bgithub.com%2FDreamacro%2Fclash.svg?type=large)](https://app.fossa.io/projects/git%2Bgithub.com%2FDreamacro%2Fclash?ref=badge_large)

## TODO

- [x] Complementing the necessary rule operators
- [x] Redir proxy
- [x] UDP support
- [ ] Connection manager
